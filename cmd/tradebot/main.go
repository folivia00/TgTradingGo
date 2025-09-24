package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"tradebot/internal/cfg"
	"tradebot/internal/core"
	"tradebot/internal/data"
	"tradebot/internal/logx"
	"tradebot/internal/risk"
	"tradebot/internal/state"
	"tradebot/internal/strategies"
	"tradebot/internal/tg"
	"tradebot/internal/web"
)

func main() {
	config := cfg.Load()
	logx.Setup(config.LogLevel)
	log.Printf("tradebot starting | mode=%s", config.Mode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tradeLog, err := data.NewTradeLog(config.TradesPath)
	if err != nil {
		log.Fatalf("trade log init: %v", err)
	}

	store := state.New(config.StatePath)
	savedState, err := store.Load()
	if err != nil {
		log.Printf("state load failed: %v", err)
	}

	var strat core.Strategy = strategies.NewEmaAtr(9, 21, 14, 1.5)
	switch savedState.Strategy.Type {
	case "ema":
		if len(savedState.Strategy.I) >= 3 && len(savedState.Strategy.F) >= 1 {
			strat = strategies.NewEmaAtr(savedState.Strategy.I[0], savedState.Strategy.I[1], savedState.Strategy.I[2], savedState.Strategy.F[0])
		}
	case "rsi":
		if len(savedState.Strategy.I) >= 1 && len(savedState.Strategy.F) >= 3 {
			strat = strategies.NewRSI(savedState.Strategy.I[0], savedState.Strategy.F[0], savedState.Strategy.F[1], savedState.Strategy.F[2])
		}
	}

	engine := core.NewEngine(core.EngineOpts{
		Mode:       config.Mode,
		EqUSD:      config.PaperEquity,
		Risk:       risk.Default(),
		NotifyFunc: func(msg string) { log.Printf("%s", msg) },
		Trades:     tradeLog,
	})
	engine.AttachStrategy(strat)

	// W1: стартуем Web сервер (dev mode допускает отсутствие initData)
	wsrv := func() *web.Server {
		dev := true     // на локалке можно оставить true; в проде — читать из ENV DEV_MODE=false
		addr := ":8080" // или из ENV WEB_ADDR
		srv := web.NewServer(config.TgToken, addr, dev)
		go func() {
			if err := srv.Serve(); err != nil {
				log.Printf("web server stopped: %v", err)
			}
		}()
		return srv
	}()
	defer wsrv.Stop()

	// TODO(W2): публиковать реальные события в SSE
	// пример: при получении свечи из feed — сериализовать в JSON и wsrv.PublishJSON(...)

	feedType := "random"
	if savedState.Feed.Type != "" {
		feedType = savedState.Feed.Type
	}

	bot := tg.NewBot(config.TgToken, engine, tradeLog, store, config.Symbol, config.TF, feedType)

	startFeed := func(ftype string) context.CancelFunc {
		ctxFeed, cancelFeed := context.WithCancel(ctx)
		switch ftype {
		case "rest":
			interval, err := time.ParseDuration(config.RestInterval)
			if err != nil || interval <= 0 {
				interval = 3 * time.Second
			}
			feed := data.NewRestFeed(config.Symbol, config.TF, interval)
			go func() {
				for k := range feed.Candles {
					if err := engine.OnCandle(k.Symbol, k.TF, k); err != nil {
						log.Printf("engine OnCandle: %v", err)
					}
				}
			}()
			feed.Start(ctxFeed)
		default:
			feed := data.NewRandomFeed(config.Symbol, config.TF, time.Now().Add(-time.Hour), 64000, 0.002)
			go func() {
				for k := range feed.Candles {
					if err := engine.OnCandle(k.Symbol, k.TF, k); err != nil {
						log.Printf("engine OnCandle: %v", err)
					}
				}
			}()
			feed.Start(ctxFeed)
		}
		return cancelFeed
	}

	var feedMu sync.Mutex
	cancelFeed := startFeed(feedType)

	go func() {
		if err := bot.Run(ctx, func(newFeed string) {
			feedMu.Lock()
			if cancelFeed != nil {
				cancelFeed()
			}
			cancelFeed = startFeed(newFeed)
			feedMu.Unlock()
		}); err != nil {
			log.Printf("telegram bot stopped: %v", err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	log.Printf("shutdown")
	cancel()
	feedMu.Lock()
	if cancelFeed != nil {
		cancelFeed()
	}
	feedMu.Unlock()
	time.Sleep(300 * time.Millisecond)
}
