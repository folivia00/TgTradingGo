package main

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/folivia00/TgTradingGo/internal/cfg"
	"github.com/folivia00/TgTradingGo/internal/core"
	"github.com/folivia00/TgTradingGo/internal/data"
	"github.com/folivia00/TgTradingGo/internal/logx"
	"github.com/folivia00/TgTradingGo/internal/risk"
	"github.com/folivia00/TgTradingGo/internal/strategies"
	"github.com/folivia00/TgTradingGo/internal/tg"
)

func main() {
	c := cfg.Load()
	logx.Setup(c.LogLevel)
	log.Info().Str("mode", c.Mode).Msg("tradebot starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Strategy
	strat := strategies.NewEmaAtr(9, 21, 14, 1.5)

	// Engine (paper mode only in this starter)
	eng := core.NewEngine(core.EngineOpts{
		Mode:       c.Mode,
		EqUSD:      c.PaperEquity,
		Risk:       risk.Default(),
		NotifyFunc: func(msg string) { log.Info().Msg(msg) },
	})

	// Telegram bot
	bot := tg.NewBot(c.TgToken, eng)
	go func() {
		if err := bot.Run(ctx); err != nil {
			log.Error().Err(err).Msg("telegram bot stopped")
		}
	}()

	// Data feed: random 1m candles for demo
	feed := data.NewRandomFeed(c.Symbol, c.TF, time.Now().Add(-time.Hour), 64000, 0.002)
}
