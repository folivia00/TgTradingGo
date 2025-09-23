package cfg

import (
	"os"
	"strconv"
)

type Config struct {
	TgToken     string
	Mode        string
	Symbol      string
	TF          string
	LogLevel    string
	PaperEquity float64
	TradesPath  string

	StatePath    string
	Exchange     string
	RestInterval string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getfloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func Load() Config {
	return Config{
		TgToken:     getenv("TG_TOKEN", ""),
		Mode:        getenv("MODE", "paper"),
		Symbol:      getenv("SYMBOL", "BTCUSDT"),
		TF:          getenv("TF", "1m"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
		PaperEquity: getfloat("PAPER_EQUITY", 10000.0),
		TradesPath:  getenv("TRADES_PATH", "trades.csv"),

		StatePath:    getenv("STATE_PATH", "state.json"),
		Exchange:     getenv("EXCHANGE", "binance"),
		RestInterval: getenv("REST_INTERVAL", "3s"),
	}
}
