package data

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"tradebot/internal/core"
)

type TradeLog struct {
	path string
	mu   sync.Mutex
}

func NewTradeLog(path string) (*TradeLog, error) {
	if path == "" {
		return nil, errors.New("empty trades path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// ensure file exists with header
	if _, err := os.Stat(abs); errors.Is(err, os.ErrNotExist) {
		f, err := os.Create(abs)
		if err != nil {
			return nil, err
		}
		w := csv.NewWriter(f)
		_ = w.Write([]string{"ts", "symbol", "tf", "event", "side", "qty", "price", "pnl", "comment"})
		w.Flush()
		_ = f.Close()
	}
	return &TradeLog{path: abs}, nil
}

func (t *TradeLog) Append(r core.TradeLogEntry) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	f, err := os.OpenFile(t.path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	rec := []string{
		r.TS.Format(time.RFC3339), r.Symbol, r.TF, r.Event, r.Side,
		formatF(r.Qty), formatF(r.Price), formatF(r.PnL), r.Comment,
	}
	if err := w.Write(rec); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

func (t *TradeLog) LastN(n int) ([]core.TradeLogEntry, error) {
	if n <= 0 {
		n = 10
	}
	f, err := os.Open(t.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	// skip header
	var out []core.TradeLogEntry
	for i := 1; i < len(rows); i++ {
		out = append(out, parseRow(rows[i]))
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out, nil
}

func parseRow(rec []string) core.TradeLogEntry {
	var ts time.Time
	ts, _ = time.Parse(time.RFC3339, rec[0])
	qty, _ := strconv.ParseFloat(rec[5], 64)
	price, _ := strconv.ParseFloat(rec[6], 64)
	pnl, _ := strconv.ParseFloat(rec[7], 64)
	return core.TradeLogEntry{TS: ts, Symbol: rec[1], TF: rec[2], Event: rec[3], Side: rec[4], Qty: qty, Price: price, PnL: pnl, Comment: rec[8]}
}

func formatF(f float64) string { return strconv.FormatFloat(f, 'f', 4, 64) }
