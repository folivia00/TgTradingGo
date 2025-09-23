package core

import "time"

type Action int

const (
	None Action = iota
	Buy
	Sell
	Close
)

type Kline struct {
	Symbol string
	TF     string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Vol    float64
	Ts     time.Time
}

type AccountState struct {
	EquityUSD float64
	Position  Position
}

type Position struct {
	Side   Action // Buy=long, Sell=short, None
	Qty    float64
	Entry  float64
	Unreal float64
}

type Signal struct {
	Action  Action
	SizePct float64
	SL      *float64
	TP      *float64
	Comment string
}

type Strategy interface {
	OnCandle(sym string, tf string, kl Kline, acct AccountState) (Signal, error)
	Warmup() int
	Name() string
}
