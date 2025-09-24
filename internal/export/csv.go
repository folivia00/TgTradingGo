package export

import (
	"encoding/csv"
	"fmt"
	"os"
)

type TradeCSV struct {
	TS              int64
	Side            string
	Qty, Price, PnL float64
	Note            string
}

type EquityCSV struct {
	TS     int64
	Equity float64
}

func WriteTradesCSV(path string, rows []TradeCSV) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"ts", "side", "qty", "price", "pnl", "note"})
	for _, r := range rows {
		w.Write([]string{itoa64(r.TS), r.Side, ftoa(r.Qty), ftoa(r.Price), ftoa(r.PnL), r.Note})
	}
	return nil
}

func WriteEquityCSV(path string, rows []EquityCSV) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"ts", "equity"})
	for _, r := range rows {
		w.Write([]string{itoa64(r.TS), ftoa(r.Equity)})
	}
	return nil
}

func itoa64(x int64) string { return fmt.Sprintf("%d", x) }
func ftoa(x float64) string { return fmt.Sprintf("%.8f", x) }
