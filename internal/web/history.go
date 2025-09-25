package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tradebot/internal/data"
)

// KlineResp описывает минимальный набор полей свечи для фронтенда.
type KlineResp struct {
	T int64   `json:"t"`
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
	V float64 `json:"v"`
}

// binIv сопоставляет таймфрейм значению из API Binance.
func binIv(tf string) string {
	switch tf {
	case "1m", "5m", "15m", "1h":
		return tf
	default:
		return "1m"
	}
}

// handleHistory отвечает за выдачу свечей за указанный период через Binance REST API.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sym := q.Get("symbol")
	tf := q.Get("tf")
	mode := q.Get("mode")
	fromStr := q.Get("from")
	toStr := q.Get("to")
	if sym == "" || tf == "" || fromStr == "" || toStr == "" {
		http.Error(w, "missing params", http.StatusBadRequest)
		return
	}

	if mode == "" {
		mode = s.DefaultExchange
	}
	if mode == "" {
		mode = "spot"
	}
	mode = strings.ToLower(mode)

	from, err1 := time.Parse(time.RFC3339, fromStr)
	to, err2 := time.Parse(time.RFC3339, toStr)
	if err1 != nil || err2 != nil {
		http.Error(w, "bad time", http.StatusBadRequest)
		return
	}

	var out []KlineResp
	if mode == "futures" {
		kl, err := data.FetchHistoryFutures(sym, tf, from, to)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		out = make([]KlineResp, 0, len(kl))
		for _, k := range kl {
			out = append(out, KlineResp{T: k.Ts.UnixMilli(), O: k.Open, H: k.High, L: k.Low, C: k.Close, V: k.Vol})
		}
	} else {
		interval := binIv(tf)
		start := from
		for start.Before(to) {
			end := start.Add(24 * time.Hour)
			if end.After(to) {
				end = to
			}

			url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1000", sym, interval, start.UnixMilli(), end.UnixMilli())
			resp, err := http.Get(url)
			if err != nil {
				http.Error(w, "fetch error", http.StatusBadGateway)
				return
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				http.Error(w, "upstream error", http.StatusBadGateway)
				return
			}

			var raw [][]any
			if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
				resp.Body.Close()
				http.Error(w, "decode", http.StatusBadGateway)
				return
			}
			resp.Body.Close()

			for _, k := range raw {
				if len(k) < 6 {
					continue
				}
				ot := toInt64(k[0])
				out = append(out, KlineResp{
					T: ot,
					O: toF64(k[1]),
					H: toF64(k[2]),
					L: toF64(k[3]),
					C: toF64(k[4]),
					V: toF64(k[5]),
				})
			}

			if len(raw) == 0 {
				break
			}
			lastOt := toInt64(raw[len(raw)-1][0])
			start = time.UnixMilli(lastOt).Add(time.Millisecond)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func toF64(v any) float64 {
	switch t := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		f, _ := strconv.ParseFloat(fmt.Sprint(v), 64)
		return f
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
		i, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64)
		return i
	}
}
