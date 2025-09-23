package strategies

import "tradebot/internal/core"

type RSI struct {
	Len        int
	Overbought float64
	Oversold   float64
	RiskR      float64

	buf  []core.Kline
	name string
}

func NewRSI(length int, over, under float64, r float64) *RSI {
	if length < 2 {
		length = 2
	}
	return &RSI{Len: length, Overbought: over, Oversold: under, RiskR: r, name: "RSI"}
}

func (s *RSI) Warmup() int  { return s.Len + 2 }
func (s *RSI) Name() string { return s.name }

func (s *RSI) OnCandle(sym, tf string, kl core.Kline, acct core.AccountState) (core.Signal, error) {
	s.buf = append(s.buf, kl)
	if len(s.buf) < s.Warmup() {
		return core.Signal{Action: core.None}, nil
	}

	cl := closes(s.buf)
	r := rsi(cl, s.Len)
	cur := r[len(r)-1]
	last := cl[len(cl)-1]

	var sig core.Signal
	if cur <= s.Oversold {
		sl := last * 0.99
		tp := last + s.RiskR*(last-sl)
		sig = core.Signal{Action: core.Buy, SizePct: 0.02, SL: &sl, TP: &tp, Comment: "rsi long"}
	} else if cur >= s.Overbought {
		sl := last * 1.01
		tp := last - s.RiskR*(sl-last)
		sig = core.Signal{Action: core.Sell, SizePct: 0.02, SL: &sl, TP: &tp, Comment: "rsi short"}
	}
	return sig, nil
}

func rsi(cl []float64, n int) []float64 {
	res := make([]float64, len(cl))
	if len(cl) == 0 {
		return res
	}
	if n < 2 {
		n = 2
	}

	gain := 0.0
	loss := 0.0
	for i := 1; i <= n && i < len(cl); i++ {
		d := cl[i] - cl[i-1]
		if d >= 0 {
			gain += d
		} else {
			loss -= d
		}
	}

	avgG := gain / float64(n)
	avgL := loss / float64(n)
	rs := 0.0
	if avgL != 0 {
		rs = avgG / avgL
	}
	res[n] = 100 - 100/(1+rs)

	for i := n + 1; i < len(cl); i++ {
		d := cl[i] - cl[i-1]
		g := 0.0
		l := 0.0
		if d >= 0 {
			g = d
		} else {
			l = -d
		}
		avgG = (avgG*float64(n-1) + g) / float64(n)
		avgL = (avgL*float64(n-1) + l) / float64(n)
		rs = 0
		if avgL != 0 {
			rs = avgG / avgL
		}
		res[i] = 100 - 100/(1+rs)
	}

	last := res[n]
	for i := 0; i < n && i < len(res); i++ {
		res[i] = last
	}
	return res
}
