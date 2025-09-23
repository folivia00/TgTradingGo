package core

import (
	"errors"
	"fmt"
	"time"
)

type Engine struct {
	mode       string
	eqUSD      float64
	risk       RiskModel
	strat      Strategy
	notifyFunc func(string)
	lastPx     float64
	trades     TradeLogger
}

type EngineOpts struct {
	Mode       string
	EqUSD      float64
	Risk       RiskModel
	NotifyFunc func(string)
	Trades     TradeLogger
}

type RiskModel interface {
	Validate(sig Signal, acct AccountState, px float64) (Signal, error)
}

func NewEngine(opts EngineOpts) *Engine {
	if opts.NotifyFunc == nil {
		opts.NotifyFunc = func(string) {}
	}
	return &Engine{mode: opts.Mode, eqUSD: opts.EqUSD, risk: opts.Risk, notifyFunc: opts.NotifyFunc, trades: opts.Trades}
}

func (e *Engine) AttachStrategy(s Strategy) { e.strat = s }
func (e *Engine) EquityUSD() float64        { return e.eqUSD }
func (e *Engine) Snapshot() AccountState    { return e.snapshot(e.lastPx) }

func (e *Engine) OnCandle(sym, tf string, kl Kline) error {
	if e.strat == nil {
		return errors.New("strategy is nil")
	}
	e.lastPx = kl.Close
	acct := e.snapshot(kl.Close)
	sig, err := e.strat.OnCandle(sym, tf, kl, acct)
	if err != nil {
		return err
	}
	if sig.Action == None {
		return nil
	}

	// Risk
	sig, err = e.risk.Validate(sig, acct, kl.Close)
	if err != nil {
		return err
	}

	// Execute (paper): naive fill at close
	ts := time.Now().UTC()
	switch sig.Action {
	case Buy:
		if acct.Position.Side == Buy { // scale-in
			qty := e.sizeUSD(sig.SizePct) / kl.Close
			avg := (acct.Position.Entry*acct.Position.Qty + kl.Close*qty) / (acct.Position.Qty + qty)
			e.notifyFunc(fmt.Sprintf("LONG add %.4f @ %.2f | TP:%v SL:%v %s", qty, kl.Close, ptrf(sig.TP), ptrf(sig.SL), sig.Comment))
			posQty += qty
			posEntry = avg
			posSide = Buy
			e.logTrade(ts, sym, tf, "ADD", Buy, qty, kl.Close, 0, sig.Comment)
		} else { // open/reverse
			qty := e.sizeUSD(sig.SizePct) / kl.Close
			e.notifyFunc(fmt.Sprintf("LONG open %.4f @ %.2f | TP:%v SL:%v %s", qty, kl.Close, ptrf(sig.TP), ptrf(sig.SL), sig.Comment))
			posSide = Buy
			posQty = qty
			posEntry = kl.Close
			e.logTrade(ts, sym, tf, "OPEN", Buy, qty, kl.Close, 0, sig.Comment)
		}
	case Sell:
		if acct.Position.Side == Sell {
			qty := e.sizeUSD(sig.SizePct) / kl.Close
			avg := (acct.Position.Entry*acct.Position.Qty + kl.Close*qty) / (acct.Position.Qty + qty)
			e.notifyFunc(fmt.Sprintf("SHORT add %.4f @ %.2f | TP:%v SL:%v %s", qty, kl.Close, ptrf(sig.TP), ptrf(sig.SL), sig.Comment))
			posQty += qty
			posEntry = avg
			posSide = Sell
			e.logTrade(ts, sym, tf, "ADD", Sell, qty, kl.Close, 0, sig.Comment)
		} else {
			qty := e.sizeUSD(sig.SizePct) / kl.Close
			e.notifyFunc(fmt.Sprintf("SHORT open %.4f @ %.2f | TP:%v SL:%v %s", qty, kl.Close, ptrf(sig.TP), ptrf(sig.SL), sig.Comment))
			posSide = Sell
			posQty = qty
			posEntry = kl.Close
			e.logTrade(ts, sym, tf, "OPEN", Sell, qty, kl.Close, 0, sig.Comment)
		}
	case Close:
		if acct.Position.Side != None {
			pnl := e.realize(kl.Close)
			e.notifyFunc(fmt.Sprintf("CLOSE @ %.2f | PnL: %.2f USD", kl.Close, pnl))
			e.logTrade(ts, sym, tf, "CLOSE", acct.Position.Side, acct.Position.Qty, kl.Close, pnl, "close")
		}
	}
	return nil
}

func (e *Engine) logTrade(ts time.Time, sym, tf, etype string, side Action, qty, price, pnl float64, comment string) {
	if e.trades == nil {
		return
	}
	_ = e.trades.Append(TradeLogEntry{
		TS:      ts,
		Symbol:  sym,
		TF:      tf,
		Event:   etype,
		Side:    map[Action]string{Buy: "LONG", Sell: "SHORT", None: "FLAT"}[side],
		Qty:     qty,
		Price:   price,
		PnL:     pnl,
		Comment: comment,
	})
}

func ptrf(p *float64) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *p)
}

func (e *Engine) snapshot(px float64) AccountState {
	return AccountState{EquityUSD: e.eqUSD, Position: e.pos(px)}
}

// Position state in paper mode (package-level simple demo)
var (
	posSide  Action
	posQty   float64
	posEntry float64
)

func (e *Engine) pos(px float64) Position {
	if posSide == None {
		return Position{}
	}
	unrl := 0.0
	if posSide == Buy {
		unrl = (px - posEntry) * posQty
	}
	if posSide == Sell {
		unrl = (posEntry - px) * posQty
	}
	return Position{Side: posSide, Qty: posQty, Entry: posEntry, Unreal: unrl}
}

func (e *Engine) realize(px float64) float64 {
	p := e.pos(px)
	if p.Side == None {
		return 0
	}
	pnl := p.Unreal
	e.eqUSD += pnl
	posSide = None
	posQty = 0
	posEntry = 0
	return pnl
}

func (e *Engine) sizeUSD(pct float64) float64 { return e.eqUSD * pct }
