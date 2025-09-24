package backtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tradebot/internal/core"
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
	StrategyKind  string             // "ema_atr" | "rsi"
	StrategyArgs  map[string]float64 // fast,slow,atr,R | len,overbought,oversold,R
}

type Trade struct {
	TS    time.Time `json:"ts"`
	Side  string    `json:"side"`
	Qty   float64   `json:"qty"`
	Price float64   `json:"price"`
	PnL   float64   `json:"pnl"`
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

type noRisk struct{}

func (noRisk) Validate(sig core.Signal, acct core.AccountState, px float64) (core.Signal, error) {
	return sig, nil
}

// Run — упрощённый бэктест: история тянется прямым REST (spot)
func Run(p Params) (Result, error) {
	// 1) загрузим историю свечей чанками (как в /api/history)
	kl, err := fetchHistorySpot(p.Symbol, p.TF, p.From, p.To)
	if err != nil {
		return Result{}, err
	}
	if len(kl) == 0 {
		return Result{}, fmt.Errorf("no history")
	}

	// 2) инициализируем движок с буферным логом сделок
	trades := make([]Trade, 0, 256)
	curTS := kl[0].Ts
	var posSide string
	var posQty float64
	notify := func(msg string) {
		// попытка распарсить из сообщения цену/направление (простая)
		t := Trade{TS: curTS, Note: msg}
		fields := strings.Fields(msg)
		if len(fields) > 0 {
			switch fields[0] {
			case "LONG", "SHORT":
				side := strings.ToLower(fields[0])
				t.Side = side
				if len(fields) >= 3 {
					if qty, err := strconv.ParseFloat(strings.Trim(fields[2], "@"), 64); err == nil {
						t.Qty = qty
						if fields[1] == "open" {
							posQty = qty
							posSide = side
						} else if fields[1] == "add" {
							posQty += qty
							posSide = side
						}
					}
				}
				if len(fields) >= 5 {
					if price, err := strconv.ParseFloat(strings.Trim(fields[4], "@"), 64); err == nil {
						t.Price = price
					}
				}
			case "CLOSE":
				t.Side = posSide
				t.Qty = posQty
				if len(fields) >= 3 {
					if price, err := strconv.ParseFloat(strings.Trim(fields[2], "@"), 64); err == nil {
						t.Price = price
					}
				}
				if idx := strings.Index(msg, "PnL:"); idx >= 0 {
					rest := msg[idx+4:]
					restFields := strings.Fields(rest)
					if len(restFields) > 0 {
						if pnl, err := strconv.ParseFloat(strings.Trim(restFields[0], ","), 64); err == nil {
							t.PnL = pnl
						}
					}
				}
				posSide = ""
				posQty = 0
			}
		}
		trades = append(trades, t)
	}
	eq := p.InitialEquity
	equity := []Point{{TS: kl[0].Ts, Equity: eq}}

	eng := core.NewEngine(core.EngineOpts{
		Mode:       "backtest",
		EqUSD:      eq,
		Risk:       noRisk{}, // отключим обрезание размера — стратегия сама вернёт sizePct
		NotifyFunc: notify,
		Trades:     nil, // не пишем в файл в режиме бэктеста
	})

	// 3) стратегия
	eng.AttachStrategy(NewStrategyFromParams(p))

	// 4) цикл по свечам
	for _, k := range kl {
		curTS = k.Ts
		ck := core.Kline{Symbol: p.Symbol, TF: p.TF, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close, Vol: k.Vol, Ts: k.Ts}
		if err := eng.OnCandle(p.Symbol, p.TF, ck); err != nil {
			return Result{}, err
		}
		s := eng.Snapshot()
		equity = append(equity, Point{TS: k.Ts, Equity: s.EquityUSD})
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
	switch p.StrategyKind {
	case "rsi":
		l := int(p.StrategyArgs["len"])
		ob := p.StrategyArgs["overbought"]
		os := p.StrategyArgs["oversold"]
		R := p.StrategyArgs["R"]
		return strategies.NewRSI(l, ob, os, R)
	default: // ema_atr
		f := int(p.StrategyArgs["fast"])
		s := int(p.StrategyArgs["slow"])
		a := int(p.StrategyArgs["atr"])
		R := p.StrategyArgs["R"]
		return strategies.NewEmaAtr(f, s, a, R)
	}
}
