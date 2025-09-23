package risk

import (
	"errors"
	"tradebot/internal/core"
)

type model struct{ maxPerTrade float64 }

func Default() core.RiskModel { return &model{maxPerTrade: 0.02} }

func (m *model) Validate(sig core.Signal, acct core.AccountState, px float64) (core.Signal, error) {
	if sig.SizePct <= 0 {
		return sig, errors.New("size pct <= 0")
	}
	if sig.SizePct > m.maxPerTrade {
		sig.SizePct = m.maxPerTrade
	}
	return sig, nil
}
