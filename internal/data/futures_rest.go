package data

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Kline struct {
	Ts              time.Time
	Open, High, Low float64
	Close           float64
	Vol             float64
}

// FetchHistoryFutures fetches Binance UM futures klines via REST.
func FetchHistoryFutures(symbol, interval string, from, to time.Time) ([]Kline, error) {
	iv := map[string]string{"1m": "1m", "5m": "5m", "15m": "15m", "1h": "1h"}[interval]
	if iv == "" {
		iv = "1m"
	}
	out := make([]Kline, 0, 4096)
	start := from
	client := &http.Client{Timeout: 10 * time.Second}
	for start.Before(to) {
		end := start.Add(24 * time.Hour)
		if end.After(to) {
			end = to
		}
		url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1500", symbol, iv, start.UnixMilli(), end.UnixMilli())
		resp, err := client.Get(url)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("binance futures status %d", resp.StatusCode)
		}
		var raw [][]any
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
		if len(raw) == 0 {
			break
		}
		for _, k := range raw {
			ts := toInt64(k[0])
			out = append(out, Kline{
				Ts:    time.UnixMilli(ts),
				Open:  toF64(k[1]),
				High:  toF64(k[2]),
				Low:   toF64(k[3]),
				Close: toF64(k[4]),
				Vol:   toF64(k[5]),
			})
		}
		last := toInt64(raw[len(raw)-1][0])
		start = time.UnixMilli(last).Add(time.Millisecond)
	}
	return out, nil
}
