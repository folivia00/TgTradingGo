package strategies

import (
	"math"

	"tradebot/internal/core"
)

type EmaAtr struct {
	Fast, Slow int
	AtrLen     int
	RiskR      float64

	buf  []core.Kline
	name string
}

func NewEmaAtr(fast, slow, atr int, r float64) *EmaAtr {
	return &EmaAtr{Fast: fast, Slow: slow, AtrLen: atr, RiskR: r, name: "EMA_ATR"}
}

func (s *EmaAtr) Warmup() int  { return max3(s.Slow, s.AtrLen, s.Fast) + 2 }
func (s *EmaAtr) Name() string { return s.name }

func (s *EmaAtr) OnCandle(sym, tf string, kl core.Kline, acct core.AccountState) (core.Signal, error) {
	s.buf = append(s.buf, kl)
	if len(s.buf) < s.Warmup() {
		return core.Signal{Action: core.None}, nil
	}

	cl := closes(s.buf)
	fast := EMA(cl, s.Fast)
	slow := EMA(cl, s.Slow)
	atr := ATR(highs(s.buf), lows(s.buf), cl, s.AtrLen)

	last := cl[len(cl)-1]
	atrv := atr[len(atr)-1]
	var sig core.Signal
	if crossUp(fast, slow) {
		sl := last - 1.5*atrv
		tp := last + s.RiskR*(last-sl)
		sig = core.Signal{Action: core.Buy, SizePct: 0.02, SL: &sl, TP: &tp, Comment: "ema up"}
	} else if crossDown(fast, slow) {
		sl := last + 1.5*atrv
		tp := last - s.RiskR*(sl-last)
		sig = core.Signal{Action: core.Sell, SizePct: 0.02, SL: &sl, TP: &tp, Comment: "ema dn"}
	}
	return sig, nil
}

// === utils ===
func EMA(x []float64, n int) []float64 {
	res := make([]float64, len(x))
	k := 2.0 / (float64(n) + 1)
	res[0] = x[0]
	for i := 1; i < len(x); i++ {
		res[i] = x[i]*k + res[i-1]*(1-k)
	}
	return res
}

func ATR(h, l, c []float64, n int) []float64 {
	tr := make([]float64, len(c))
	tr[0] = h[0] - l[0]
	for i := 1; i < len(c); i++ {
		m1 := h[i] - l[i]
		m2 := math.Abs(h[i] - c[i-1])
		m3 := math.Abs(l[i] - c[i-1])
		tr[i] = max3f(m1, m2, m3)
	}
	return EMA(tr, n)
}

func crossUp(a, b []float64) bool {
	if len(a) < 2 || len(b) < 2 {
		return false
	}
	return a[len(a)-2] <= b[len(b)-2] && a[len(a)-1] > b[len(b)-1]
}

func crossDown(a, b []float64) bool {
	if len(a) < 2 || len(b) < 2 {
		return false
	}
	return a[len(a)-2] >= b[len(b)-2] && a[len(a)-1] < b[len(b)-1]
}

func closes(k []core.Kline) []float64 {
	r := make([]float64, len(k))
	for i := range k {
		r[i] = k[i].Close
	}
	return r
}
func highs(k []core.Kline) []float64 {
	r := make([]float64, len(k))
	for i := range k {
		r[i] = k[i].High
	}
	return r
}
func lows(k []core.Kline) []float64 {
	r := make([]float64, len(k))
	for i := range k {
		r[i] = k[i].Low
	}
	return r
}

func max3(a, b, c int) int {
	if a < b {
		a = b
	}
	if a < c {
		a = c
	}
	return a
}
func max3f(a, b, c float64) float64 {
	if a < b {
		a = b
	}
	if a < c {
		a = c
	}
	return a
}
