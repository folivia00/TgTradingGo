package main

import (
	"context"
	"encoding/json"
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
	c := cfg.Load()
	logx.Setup(c.LogLevel)
	log.Printf("tradebot starting | mode=%s", c.Mode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tl, err := data.NewTradeLog(c.TradesPath)
	if err != nil {
		log.Fatalf("trade log init: %v", err)
	}

	store := state.New(c.StatePath)
	st, err := store.Load()
	if err != nil {
		log.Printf("state load failed: %v", err)
	}

	var strat core.Strategy = strategies.NewEmaAtr(9, 21, 14, 1.5)
	switch st.Strategy.Type {
	case "ema":
		if len(st.Strategy.I) >= 3 && len(st.Strategy.F) >= 1 {
			strat = strategies.NewEmaAtr(st.Strategy.I[0], st.Strategy.I[1], st.Strategy.I[2], st.Strategy.F[0])
		}
	case "rsi":
		if len(st.Strategy.I) >= 1 && len(st.Strategy.F) >= 3 {
			strat = strategies.NewRSI(st.Strategy.I[0], st.Strategy.F[0], st.Strategy.F[1], st.Strategy.F[2])
		}
	}

	wsrv := web.NewServer(c.TgToken, web.EnvAddr(), web.EnvDev())

	eng := core.NewEngine(core.EngineOpts{
		Mode:  c.Mode,
		EqUSD: c.PaperEquity,
		Risk:  risk.Default(),
		NotifyFunc: func(msg string) {
			log.Printf("%s", msg)
			if line, err := json.Marshal(map[string]any{
				"type": "trade",
				"data": map[string]any{"msg": msg},
			}); err == nil {
				wsrv.PublishJSON(string(line))
			} else {
				log.Printf("notify marshal: %v", err)
			}
		},
		Trades: tl,
	})
	eng.AttachStrategy(strat)

	go func() {
		if err := wsrv.Serve(); err != nil {
			log.Printf("web server stopped: %v", err)
		}
	}()

	feedType := "random"
	if st.Feed.Type != "" {
		feedType = st.Feed.Type
	}

	startFeed := func(ftype string) context.CancelFunc {
		ctxFeed, cancelFeed := context.WithCancel(ctx)
		switch ftype {
		case "rest":
			interval, err := time.ParseDuration(c.RestInterval)
			if err != nil || interval <= 0 {
				interval = 3 * time.Second
			}
			feed := data.NewRestFeed(c.Symbol, c.TF, interval)
			go func() {
				for k := range feed.Candles {
					if line, err := json.Marshal(map[string]any{
						"type": "candle",
						"data": map[string]any{
							"t": k.Ts.UnixMilli(),
							"o": k.Open,
							"h": k.High,
							"l": k.Low,
							"c": k.Close,
						},
					}); err == nil {
						wsrv.PublishJSON(string(line))
					} else {
						log.Printf("candle marshal: %v", err)
					}
					if err := eng.OnCandle(k.Symbol, k.TF, k); err != nil {
						log.Printf("engine OnCandle: %v", err)
					}
				}
			}()
			feed.Start(ctxFeed)
		default:
			feed := data.NewRandomFeed(c.Symbol, c.TF, time.Now().Add(-time.Hour), 64000, 0.002)
			go func() {
				for k := range feed.Candles {
					if line, err := json.Marshal(map[string]any{
						"type": "candle",
						"data": map[string]any{
							"t": k.Ts.UnixMilli(),
							"o": k.Open,
							"h": k.High,
							"l": k.Low,
							"c": k.Close,
						},
					}); err == nil {
						wsrv.PublishJSON(string(line))
					} else {
						log.Printf("candle marshal: %v", err)
					}
					if err := eng.OnCandle(k.Symbol, k.TF, k); err != nil {
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

	bot := tg.NewBot(c.TgToken, eng, tl, store, c.Symbol, c.TF, feedType)

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
	wsrv.Stop()
	time.Sleep(300 * time.Millisecond)
}
