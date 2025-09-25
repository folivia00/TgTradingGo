package core

import "time"

type TradeLogEntry struct {
	TS      time.Time
	Symbol  string
	TF      string
	Event   string
	Side    string
	Qty     float64
	Price   float64
	PnL     float64
	Fee     float64
	Comment string
}

type TradeLogger interface {
	Append(TradeLogEntry) error
	LastN(n int) ([]TradeLogEntry, error)
}
