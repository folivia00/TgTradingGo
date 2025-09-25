package backtest

import "strings"

type pair struct{ G, L float64 }

func ComputeMetrics(eq []Point, trades []Trade) Summary {
	var sumPnL float64
	var pf pair
	wins := 0
	closes := 0
	for _, t := range trades {
		if strings.ToUpper(t.Event) != "CLOSE" {
			continue
		}
		closes++
		sumPnL += t.PnL
		if t.PnL >= 0 {
			pf.G += t.PnL
			if t.PnL > 0 {
				wins++
			}
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
	if closes > 0 {
		wr = float64(wins) / float64(closes)
	}
	pfv := 0.0
	if pf.L > 0 {
		pfv = pf.G / pf.L
	}
	return Summary{PNL: sumPnL, Trades: closes, WinRate: wr, ProfitFact: pfv, MaxDD: dd}
}
