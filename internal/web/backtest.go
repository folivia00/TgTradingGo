package web

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tradebot/internal/backtest"
	"tradebot/internal/export"
)

type btReq struct {
	Symbol        string              `json:"symbol"`
	TF            string              `json:"tf"`
	From          string              `json:"from"` // RFC3339
	To            string              `json:"to"`
	InitialEquity float64             `json:"initialEquity"`
	Leverage      float64             `json:"leverage"`
	SlippageBps   float64             `json:"slippageBps"`
	Fees          backtest.FeesConfig `json:"fees"`
	StrategyKind  string              `json:"strategy"`
	StrategyArgs  map[string]float64  `json:"args"`
}

type btResp struct {
	Summary   backtest.Summary  `json:"summary"`
	Artifacts map[string]string `json:"artifacts"` // ключи: zip, equity_svg, price_svg
	ID        string            `json:"id"`
}

type store struct {
	mu sync.Mutex
	m  map[string]string
}

var art = &store{m: map[string]string{}}

func (s *Server) handleBacktest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var req btReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	from, err1 := time.Parse(time.RFC3339, req.From)
	to, err2 := time.Parse(time.RFC3339, req.To)
	if err1 != nil || err2 != nil || !to.After(from) {
		http.Error(w, "bad dates", 400)
		return
	}
	if req.InitialEquity <= 0 {
		req.InitialEquity = 10000
	}
	if req.Leverage <= 0 {
		req.Leverage = 1
	}
	if req.StrategyKind == "" {
		req.StrategyKind = "ema_atr"
	}

	p := backtest.Params{
		Symbol: req.Symbol, TF: req.TF, From: from, To: to,
		InitialEquity: req.InitialEquity, Leverage: req.Leverage, SlippageBps: req.SlippageBps,
		Fees: req.Fees, StrategyKind: req.StrategyKind, StrategyArgs: req.StrategyArgs,
	}
	res, err := backtest.Run(p)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// сохранение артефактов
	id := fmt.Sprintf("bt_%d", time.Now().UnixNano())
	base := filepath.Join(os.TempDir(), "tradebot", id)
	if err := export.EnsureDir(base); err != nil {
		http.Error(w, "mk dir", 500)
		return
	}

	// CSV
	trcsv := export.Join(base, "trades.csv")
	eqcsv := export.Join(base, "equity.csv")
	{
		rows := make([]export.TradeCSV, 0, len(res.Trades))
		for _, t := range res.Trades {
			rows = append(rows, export.TradeCSV{TS: t.TS.UnixMilli(), Side: t.Side, Qty: t.Qty, Price: t.Price, PnL: t.PnL, Note: t.Note})
		}
		if err := export.WriteTradesCSV(trcsv, rows); err != nil {
			http.Error(w, "write trades", 500)
			return
		}
		pts := make([]export.EquityCSV, 0, len(res.EquityCurve))
		for _, p := range res.EquityCurve {
			pts = append(pts, export.EquityCSV{TS: p.TS.UnixMilli(), Equity: p.Equity})
		}
		if err := export.WriteEquityCSV(eqcsv, pts); err != nil {
			http.Error(w, "write equity", 500)
			return
		}
	}

	// SVG простые: из equity и из цены (возьмём close по точкам equity как плейсхолдер)
	reqsvg := export.Join(base, "equity.svg")
	rprsvg := export.Join(base, "price.svg")
	{
		line := make([]export.Line, 0, len(res.EquityCurve))
		for i, p := range res.EquityCurve {
			line = append(line, export.Line{X: float64(i), Y: p.Equity})
		}
		if err := os.WriteFile(reqsvg, export.SimpleSVGChart(900, 300, line, nil, "Equity"), 0o644); err != nil {
			http.Error(w, "write equity svg", 500)
			return
		}
		// price mock: нормируем equity к 100..200 просто для превью; на W4 заменим на реальную цену
		min, max := math.MaxFloat64, -math.MaxFloat64
		for _, p := range res.EquityCurve {
			if p.Equity < min {
				min = p.Equity
			}
			if p.Equity > max {
				max = p.Equity
			}
		}
		line2 := make([]export.Line, 0, len(res.EquityCurve))
		for i, p := range res.EquityCurve {
			y := 100 + (p.Equity-min)/(max-min+1e-9)*100
			line2 = append(line2, export.Line{X: float64(i), Y: y})
		}
		if err := os.WriteFile(rprsvg, export.SimpleSVGChart(900, 300, line2, nil, "Price (preview)"), 0o644); err != nil {
			http.Error(w, "write price svg", 500)
			return
		}
	}

	// HTML
	html := export.Join(base, "report.html")
	if err := os.WriteFile(html, backtest.HTMLReport("Backtest Report", res.Summary, "/api/export?id="+id), 0o644); err != nil {
		http.Error(w, "write html", 500)
		return
	}

	// ZIP
	zipPath := filepath.Join(base, "report.zip")
	if err := export.ZipFiles(zipPath, map[string]string{
		"trades.csv":  trcsv,
		"equity.csv":  eqcsv,
		"equity.svg":  reqsvg,
		"price.svg":   rprsvg,
		"report.html": html,
	}); err != nil {
		http.Error(w, "zip", 500)
		return
	}
	art.mu.Lock()
	art.m[id] = zipPath
	art.mu.Unlock()

	out := btResp{Summary: res.Summary, ID: id, Artifacts: map[string]string{"zip": "/api/export?id=" + id, "equity_svg": "/api/file?id=" + id + "&name=equity.svg", "price_svg": "/api/file?id=" + id + "&name=price.svg"}}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id", 400)
		return
	}
	art.mu.Lock()
	path := art.m[id]
	art.mu.Unlock()
	if path == "" {
		http.Error(w, "not found", 404)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=report.zip")
	http.ServeFile(w, r, path)
}

// опционально — раздать отдельный файл из папки артефактов (svg/png и т.п.)
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	name := r.URL.Query().Get("name")
	if id == "" || name == "" {
		http.Error(w, "params", 400)
		return
	}
	base := filepath.Join(os.TempDir(), "tradebot", id)
	path := filepath.Join(base, name)
	http.ServeFile(w, r, path)
}
