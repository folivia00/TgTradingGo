package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"tradebot/internal/core"
)

type RestFeed struct {
	Symbol   string
	TF       string
	Interval time.Duration
	Candles  chan core.Kline
	client   *http.Client
}

func NewRestFeed(symbol, tf string, interval time.Duration) *RestFeed {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return &RestFeed{
		Symbol:   symbol,
		TF:       tf,
		Interval: interval,
		Candles:  make(chan core.Kline, 1000),
		client:   &http.Client{Timeout: 8 * time.Second},
	}
}

func (f *RestFeed) Start(ctx context.Context) {
	go func() {
		defer close(f.Candles)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			k, err := f.fetchLast()
			if err == nil {
				f.Candles <- k
			}

			t := f.Interval
			if t <= 0 {
				t = 3 * time.Second
			}
			time.Sleep(t)
		}
	}()
}

func (f *RestFeed) fetchLast() (core.Kline, error) {
	interval := tfToBinance(f.TF)
	url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&limit=1", f.Symbol, interval)
	resp, err := f.client.Get(url)
	if err != nil {
		return core.Kline{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return core.Kline{}, fmt.Errorf("binance status %d", resp.StatusCode)
	}
	var payload [][]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return core.Kline{}, err
	}
	if len(payload) == 0 {
		return core.Kline{}, errors.New("empty klines")
	}
	row := payload[0]
	openTime := toInt64(row[0])
	open := toF64(row[1])
	high := toF64(row[2])
	low := toF64(row[3])
	close := toF64(row[4])
	vol := toF64(row[5])
	return core.Kline{
		Symbol: f.Symbol,
		TF:     f.TF,
		Open:   open,
		High:   high,
		Low:    low,
		Close:  close,
		Vol:    vol,
		Ts:     time.UnixMilli(openTime),
	}, nil
}

func tfToBinance(tf string) string {
	switch tf {
	case "1m":
		return "1m"
	case "5m":
		return "5m"
	case "15m":
		return "15m"
	case "1h":
		return "1h"
	default:
		return "1m"
	}
}

func toF64(v any) float64 {
	switch t := v.(type) {
	case string:
		var x float64
		fmt.Sscanf(t, "%f", &x)
		return x
	case float64:
		return t
	default:
		var x float64
		fmt.Sscanf(fmt.Sprint(v), "%f", &x)
		return x
	}
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case json.Number:
		i, _ := t.Int64()
		return i
	default:
		var x int64
		fmt.Sscanf(fmt.Sprint(v), "%d", &x)
		return x
	}
}
