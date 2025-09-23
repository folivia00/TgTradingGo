package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"tradebot/internal/cfg"
	"tradebot/internal/core"
	"tradebot/internal/data"
	"tradebot/internal/logx"
	"tradebot/internal/risk"
	"tradebot/internal/strategies"
	"tradebot/internal/tg"
)

func main() {
	c := cfg.Load()
	logx.Setup(c.LogLevel)
	log.Info().Str("mode", c.Mode).Msg("tradebot starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Strategy
	strat := strategies.NewEmaAtr(9, 21, 14, 1.5)

	// Engine (paper demo)
	eng := core.NewEngine(core.EngineOpts{
		Mode:       c.Mode,
		EqUSD:      c.PaperEquity,
		Risk:       risk.Default(),
		NotifyFunc: func(msg string) { log.Info().Msg(msg) },
	})
	eng.AttachStrategy(strat)

	// Telegram bot
	bot := tg.NewBot(c.TgToken, eng)
	go func() {
		if err := bot.Run(ctx); err != nil {
			log.Error().Err(err).Msg("telegram bot stopped")
		}
	}()

	// Data feed: random 1m candles for demo
	feed := data.NewRandomFeed(c.Symbol, c.TF, time.Now().Add(-time.Hour), 64000, 0.002)
	go func() {
		for k := range feed.Candles {
			if err := eng.OnCandle(k.Symbol, k.TF, k); err != nil {
				log.Error().Err(err).Msg("engine OnCandle")
			}
		}
	}()
	feed.Start(ctx)

	// Graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	log.Info().Msg("shutdown")
	cancel()
	time.Sleep(300 * time.Millisecond)
}
