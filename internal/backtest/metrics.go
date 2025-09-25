package backtest

type pair struct{ G, L float64 }

func ComputeMetrics(eq []Point, trades []Trade) Summary {
	var sumPnL float64
	var pf pair
	for _, t := range trades {
		sumPnL += t.PnL
		if t.PnL >= 0 {
			pf.G += t.PnL
		} else {
			pf.L += -t.PnL
		}
	}
	// max drawdown по equity
	var peak, dd float64
	if len(eq) > 0 {
		peak = eq[0].Equity
	}
	for _, p := range eq {
		if p.Equity > peak {
			peak = p.Equity
		}
		d := (p.Equity - peak) / peak * 100
		if d < dd {
			dd = d
		}
	}
	wr := 0.0
	if n := len(trades); n > 0 {
		w := 0
		for _, t := range trades {
			if t.PnL > 0 {
				w++
			}
		}
		wr = float64(w) / float64(n)
	}
	pfv := 0.0
	if pf.L > 0 {
		pfv = pf.G / pf.L
	}
	return Summary{PNL: sumPnL, Trades: len(trades), WinRate: wr, ProfitFact: pfv, MaxDD: dd}
}
