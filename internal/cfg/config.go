package cfg

import (
	"github.com/spf13/viper"
)

type Config struct {
	TgToken     string
	Mode        string // paper | live
	Symbol      string
	TF          string
	LogLevel    string
	PaperEquity float64
}

func Load() Config {
	v := viper.New()
	v.SetConfigFile(".env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("MODE", "paper")
	v.SetDefault("SYMBOL", "BTCUSDT")
	v.SetDefault("TF", "1m")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("PAPER_EQUITY", 10000.0)

	return Config{
		TgToken:     v.GetString("TG_TOKEN"),
		Mode:        v.GetString("MODE"),
		Symbol:      v.GetString("SYMBOL"),
		TF:          v.GetString("TF"),
		LogLevel:    v.GetString("LOG_LEVEL"),
		PaperEquity: v.GetFloat64("PAPER_EQUITY"),
	}
}
