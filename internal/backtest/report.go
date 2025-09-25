package backtest

import (
	"bytes"
	"fmt"
)

func HTMLReport(title string, sum Summary, zipName string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<!doctype html><html><head><meta charset='utf-8'><title>%s</title>", title)
	b.WriteString("<style>body{font-family:Inter,system-ui,sans-serif;padding:16px;background:#0b0f17;color:#e6edf3}table{border-collapse:collapse}td,th{border:1px solid #1f2837;padding:6px 8px}</style>")
	b.WriteString("</head><body>")
	fmt.Fprintf(&b, "<h2>%s</h2>", title)
	fmt.Fprintf(&b, "<p>PNL: <b>%.2f</b> | Trades: <b>%d</b> | WinRate: <b>%.1f%%</b> | PF: <b>%.2f</b> | MaxDD: <b>%.2f%%</b></p>", sum.PNL, sum.Trades, sum.WinRate*100, sum.ProfitFact, sum.MaxDD)
	fmt.Fprintf(&b, "<p><a href='%s'>Download ZIP</a></p>", zipName)
	b.WriteString("</body></html>")
	return b.Bytes()
}
