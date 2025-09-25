package backtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tradebot/internal/core"
	"tradebot/internal/data"
	"tradebot/internal/strategies"
)

type FeesConfig struct{ MakerBps, TakerBps float64 }

type Params struct {
	Symbol        string
	TF            string
	From          time.Time
	To            time.Time
	InitialEquity float64
	Leverage      float64
	SlippageBps   float64
	Fees          FeesConfig
	Exchange      string
	StrategyKind  string         // "ema_atr" | "rsi" | "dsl"
	StrategyArgs  map[string]any // params for strategy (numbers or ids)
}

type Trade struct {
	TS    time.Time `json:"ts"`
	Event string    `json:"event"`
	Side  string    `json:"side"`
	Qty   float64   `json:"qty"`
	Price float64   `json:"price"`
	PnL   float64   `json:"pnl"`
	Fee   float64   `json:"fee"`
	Note  string    `json:"note"`
}

type Point struct {
	TS     time.Time `json:"ts"`
	Equity float64   `json:"eq"`
}

type Result struct {
	Trades      []Trade `json:"trades"`
	EquityCurve []Point `json:"equity"`
	Summary     Summary `json:"summary"`
}

type Summary struct {
	PNL        float64 `json:"pnl"`
	Trades     int     `json:"trades"`
	WinRate    float64 `json:"winRate"`
	ProfitFact float64 `json:"profitFactor"`
	MaxDD      float64 `json:"maxDD"`
}

type leverageRisk struct{ leverage float64 }

func (lr leverageRisk) Validate(sig core.Signal, acct core.AccountState, px float64) (core.Signal, error) {
	if lr.leverage <= 1 {
		return sig, nil
	}
	if sig.Action == core.Buy || sig.Action == core.Sell {
		sig.SizePct *= lr.leverage
		if sig.SizePct > lr.leverage {
			sig.SizePct = lr.leverage
		}
	}
	return sig, nil
}

// Run — упрощённый бэктест: история тянется прямым REST (spot)
func Run(p Params) (Result, error) {
	exch := strings.ToLower(p.Exchange)
	// 1) загрузим историю свечей
	var kl []kline
	var err error
	if exch == "futures" {
		fr, ferr := data.FetchHistoryFutures(p.Symbol, p.TF, p.From, p.To)
		err = ferr
		if err == nil {
			kl = make([]kline, len(fr))
			for i, v := range fr {
				kl[i] = kline{Ts: v.Ts, Open: v.Open, High: v.High, Low: v.Low, Close: v.Close, Vol: v.Vol}
			}
		}
	} else {
		kl, err = fetchHistorySpot(p.Symbol, p.TF, p.From, p.To)
	}
	if err != nil {
		return Result{}, err
	}
	if len(kl) == 0 {
		return Result{}, fmt.Errorf("no history")
	}

	// 2) инициализируем движок с буферным логом сделок
	trades := make([]Trade, 0, 256)
	feeAccTotal := 0.0
	roundTripFees := 0.0
	feeRate := p.Fees.TakerBps / 10000.0
	if feeRate < 0 {
		feeRate = 0
	}

	eq := p.InitialEquity
	equity := []Point{{TS: kl[0].Ts, Equity: eq}}

	eng := core.NewEngine(core.EngineOpts{
		Mode:       "backtest",
		EqUSD:      eq,
		Risk:       leverageRisk{leverage: p.Leverage},
		NotifyFunc: func(string) {},
		TradeHook: func(ev core.TradeEvent) {
			if ev.Qty <= 0 || ev.Price <= 0 {
				return
			}
			fee := ev.Price * ev.Qty * feeRate
			if fee < 0 {
				fee = 0
			}
			feeAccTotal += fee

			switch strings.ToUpper(ev.Event) {
			case "OPEN", "ADD":
				roundTripFees += fee
			case "CLOSE":
				totalFee := roundTripFees + fee
				netPnL := ev.PnL - totalFee
				trades = append(trades, Trade{
					TS:    ev.TS,
					Event: ev.Event,
					Side:  actionToSide(ev.Side),
					Qty:   ev.Qty,
					Price: ev.Price,
					PnL:   netPnL,
					Fee:   totalFee,
					Note:  ev.Comment,
				})
				roundTripFees = 0
			}
		},
		Trades: nil, // не пишем в файл в режиме бэктеста
	})

	// 3) стратегия
	eng.AttachStrategy(NewStrategyFromParams(p))

	// 4) цикл по свечам
	for _, k := range kl {
		ck := core.Kline{Symbol: p.Symbol, TF: p.TF, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close, Vol: k.Vol, Ts: k.Ts}
		if err := eng.OnCandle(p.Symbol, p.TF, ck); err != nil {
			return Result{}, err
		}
		s := eng.Snapshot()
		equity = append(equity, Point{TS: k.Ts, Equity: s.EquityUSD - feeAccTotal})
	}

	// 5) метрики
	sm := ComputeMetrics(equity, trades)

	return Result{Trades: trades, EquityCurve: equity, Summary: sm}, nil
}

// ===== История (Spot) — как в W2, но как внутренняя функция

type kline struct {
	Ts                          time.Time
	Open, High, Low, Close, Vol float64
}

func fetchHistorySpot(sym, tf string, from, to time.Time) ([]kline, error) {
	iv := map[string]string{"1m": "1m", "5m": "5m", "15m": "15m", "1h": "1h"}[tf]
	if iv == "" {
		iv = "1m"
	}
	out := make([]kline, 0, 4096)
	start := from
	for start.Before(to) {
		end := start.Add(24 * time.Hour)
		if end.After(to) {
			end = to
		}
		url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1000", sym, iv, start.UnixMilli(), end.UnixMilli())
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		var raw [][]any
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
		if len(raw) == 0 {
			break
		}
		for _, k := range raw {
			ot := toInt64(k[0])
			out = append(out, kline{Ts: time.UnixMilli(ot), Open: toF64(k[1]), High: toF64(k[2]), Low: toF64(k[3]), Close: toF64(k[4]), Vol: toF64(k[5])})
		}
		last := toInt64(raw[len(raw)-1][0])
		start = time.UnixMilli(last).Add(1 * time.Millisecond)
	}
	return out, nil
}

func toF64(v any) float64 {
	switch t := v.(type) {
	case string:
		var x float64
		fmt.Sscanf(t, "%f", &x)
		return x
	case float64:
		return t
	default:
		var x float64
		fmt.Sscanf(fmt.Sprint(v), "%f", &x)
		return x
	}
}
func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	default:
		var x int64
		fmt.Sscanf(fmt.Sprint(v), "%d", &x)
		return x
	}
}

// NewStrategyFromParams — адаптер: соберёт реализацию core.Strategy из Params
func NewStrategyFromParams(p Params) core.Strategy {
	kind := strings.ToLower(p.StrategyKind)
	args := p.StrategyArgs
	if args == nil {
		args = map[string]any{}
	}
	switch kind {
	case "dsl":
		id := ""
		if v, ok := args["id"]; ok {
			id = fmt.Sprint(v)
		}
		if id == "" {
			id = SelectedDSL()
		}
		if doc, ok := GetDSLDoc(id); ok {
			if spec, err := compileDSL(doc.Body); err == nil {
				compiledArgs := make(map[string]any, len(spec.Args))
				for k, v := range spec.Args {
					compiledArgs[k] = v
				}
				return strategyFromKind(spec.Kind, compiledArgs)
			}
		}
		kind = "ema_atr"
	case "", "ema", "ema_atr":
		kind = "ema_atr"
	}
	return strategyFromKind(kind, args)
}

func strategyFromKind(kind string, args map[string]any) core.Strategy {
	switch strings.ToLower(kind) {
	case "rsi":
		l := intArg(args, "len", 14)
		ob := numArg(args, "overbought", 70)
		os := numArg(args, "oversold", 30)
		R := numArg(args, "R", 1.5)
		return strategies.NewRSI(l, ob, os, R)
	default:
		f := intArg(args, "fast", 9)
		s := intArg(args, "slow", 21)
		a := intArg(args, "atr", 14)
		R := numArg(args, "R", 1.5)
		return strategies.NewEmaAtr(f, s, a, R)
	}
}

func numArg(args map[string]any, key string, def float64) float64 {
	v, ok := args[key]
	if !ok {
		lower := strings.ToLower(key)
		upper := strings.ToUpper(key)
		if alt, okAlt := args[lower]; okAlt {
			v = alt
			ok = true
		} else if alt, okAlt := args[upper]; okAlt {
			v = alt
			ok = true
		}
	}
	if !ok {
		return def
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case json.Number:
		f, err := val.Float64()
		if err == nil {
			return f
		}
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	default:
		if f, err := strconv.ParseFloat(fmt.Sprint(v), 64); err == nil {
			return f
		}
	}
	return def
}

func intArg(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok {
		lower := strings.ToLower(key)
		upper := strings.ToUpper(key)
		if alt, okAlt := args[lower]; okAlt {
			v = alt
			ok = true
		} else if alt, okAlt := args[upper]; okAlt {
			v = alt
			ok = true
		}
	}
	if !ok {
		return def
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return int(i)
		}
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return int(numArg(args, key, float64(def)))
}

func actionToSide(a core.Action) string {
	switch a {
	case core.Buy:
		return "long"
	case core.Sell:
		return "short"
	default:
		return "flat"
	}
}
