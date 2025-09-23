package data


import (
"context"
"math/rand"
"time"


"github.com/yourname/tradebot/internal/core"
)


type RandomFeed struct {
Symbol string
TF string
start time.Time
price float64
vol float64
Candles chan core.Kline
}


func NewRandomFeed(symbol, tf string, start time.Time, startPrice float64, vol float64) *RandomFeed {
return &RandomFeed{Symbol:symbol, TF:tf, start:start, price:startPrice, vol:vol, Candles: make(chan core.Kline, 1000)}
}


func (f *RandomFeed) Start(ctx context.Context) {
step := tfDur(f.TF)
if step <= 0 { step = time.Minute }
go func() {
defer close(f.Candles)
ts := f.start
r := rand.New(rand.NewSource(time.Now().UnixNano()))
for {
select { case <-ctx.Done(): return default:
}
open := f.price
ret := (r.Float64()-0.5)*2.0*f.vol // +/- vol
close := open * (1.0 + ret)
high := maxf(open, close) * (1.0 + r.Float64()*f.vol*0.5)
low := minf(open, close) * (1.0 - r.Float64()*f.vol*0.5)
vol := 10_000 + r.Float64()*5_000
k := core.Kline{Symbol:f.Symbol, TF:f.TF, Open:open, High:high, Low:low, Close:close, Vol:vol, Ts:ts}
f.Candles <- k
f.price = close
ts = ts.Add(step)
time.Sleep(100 * time.Millisecond) // fast-forward 1m candle each 0.1s
}
}()
}


func tfDur(tf string) time.Duration {
switch tf { case "1m": return time.Minute; case "5m": return 5*time.Minute; case "15m": return 15*time.Minute; }
return time.Minute
}


func maxf(a,b float64) float64 { if a<b {return b}; return a }
func minf(a,b float64) float64 { if a<b {return a}; return b }