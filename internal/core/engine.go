package core
case Buy:
if acct.Position.Side == Buy { // scale-in
qty := e.sizeUSD(sig.SizePct) / kl.Close
avg := (acct.Position.Entry*acct.Position.Qty + kl.Close*qty) / (acct.Position.Qty + qty)
eqChange := 0.0
e.eqUSD += eqChange
e.notifyFunc(fmt.Sprintf("LONG add %.4f @ %.2f | TP:%v SL:%v %s", qty, kl.Close, ptrf(sig.TP), ptrf(sig.SL), sig.Comment))
acct.Position.Qty += qty
acct.Position.Entry = avg
} else { // open/reverse
qty := e.sizeUSD(sig.SizePct) / kl.Close
e.notifyFunc(fmt.Sprintf("LONG open %.4f @ %.2f | TP:%v SL:%v %s", qty, kl.Close, ptrf(sig.TP), ptrf(sig.SL), sig.Comment))
e.setPos(Buy, qty, kl.Close)
}
case Sell:
if acct.Position.Side == Sell {
qty := e.sizeUSD(sig.SizePct) / kl.Close
avg := (acct.Position.Entry*acct.Position.Qty + kl.Close*qty) / (acct.Position.Qty + qty)
e.notifyFunc(fmt.Sprintf("SHORT add %.4f @ %.2f | TP:%v SL:%v %s", qty, kl.Close, ptrf(sig.TP), ptrf(sig.SL), sig.Comment))
acct.Position.Qty += qty
acct.Position.Entry = avg
} else {
qty := e.sizeUSD(sig.SizePct) / kl.Close
e.notifyFunc(fmt.Sprintf("SHORT open %.4f @ %.2f | TP:%v SL:%v %s", qty, kl.Close, ptrf(sig.TP), ptrf(sig.SL), sig.Comment))
e.setPos(Sell, qty, kl.Close)
}
case Close:
if acct.Position.Side != None {
pnl := e.realize(kl.Close)
e.notifyFunc(fmt.Sprintf("CLOSE @ %.2f | PnL: %.2f USD", kl.Close, pnl))
}
}
return nil
}


func ptrf(p *float64) string {
if p == nil { return "-" }
return fmt.Sprintf("%.2f", *p)
}


func (e *Engine) snapshot(px float64) AccountState {
return AccountState{EquityUSD: e.eqUSD, Position: e.pos(px)}
}


// Position state in paper mode
var (
posSide Action
posQty float64
posEntry float64
)


func (e *Engine) pos(px float64) Position {
if posSide == None { return Position{} }
unrl := 0.0
if posSide == Buy { unrl = (px - posEntry) * posQty }
if posSide == Sell { unrl = (posEntry - px) * posQty }
return Position{Side: posSide, Qty: posQty, Entry: posEntry, Unreal: unrl}
}


func (e *Engine) setPos(side Action, qty, px float64) {
posSide = side
posQty = qty
posEntry = px
}


func (e *Engine) realize(px float64) float64 {
p := e.pos(px)
if p.Side == None { return 0 }
pnl := p.Unreal
e.eqUSD += pnl
posSide = None
posQty = 0
posEntry = 0
return pnl
}


func (e *Engine) sizeUSD(pct float64) float64 { return e.eqUSD * pct }


func (e *Engine) SetNotify(f func(string)) { e.notifyFunc = f }


func (e *Engine) AttachStrategy(s Strategy) { e.strat = s }