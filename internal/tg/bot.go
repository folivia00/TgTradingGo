package tg

import (
	"context"
	"fmt"
	"strings"

	gobot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
	"tradebot/internal/core"
	"tradebot/internal/strategies"
)

type Bot struct {
	token string
	eng   *core.Engine
}

func NewBot(token string, eng *core.Engine) *Bot { return &Bot{token: token, eng: eng} }

func (b *Bot) Run(ctx context.Context) error {
	if b.token == "" {
		log.Warn().Msg("TG token empty: bot disabled")
		return nil
	}
	bot, err := gobot.NewBotAPI(b.token)
	if err != nil {
		return err
	}
	bot.Debug = false
	log.Info().Str("@", bot.Self.UserName).Msg("Telegram connected")

	u := gobot.NewUpdate(0)
	u.Timeout = 30

	updates := bot.GetUpdatesChan(u)
	for {
		select {
		case <-ctx.Done():
			return nil
		case up := <-updates:
			if up.Message == nil {
				continue
			}
			chatID := up.Message.Chat.ID
			text := strings.TrimSpace(up.Message.Text)
			switch {
			case strings.HasPrefix(text, "/start"):
				b.reply(bot, chatID, "Привет! Команды: /status, /start_trading, /stop_trading, /set_strategy ema <fast> <slow> <atr> <R>")
			case strings.HasPrefix(text, "/status"):
				b.reply(bot, chatID, b.status())
			case strings.HasPrefix(text, "/start_trading"):
				// В этой демо-версии просто сообщаем: в проде можно переключать NotifyFunc в engine
				b.reply(bot, chatID, "Торговля включена (paper)")
			case strings.HasPrefix(text, "/stop_trading"):
				b.reply(bot, chatID, "Торговля выключена")
			case strings.HasPrefix(text, "/set_strategy"):
				// /set_strategy ema 9 21 14 1.5
				parts := strings.Fields(text)
				if len(parts) >= 6 && parts[1] == "ema" {
					f := atoi(parts[2])
					s := atoi(parts[3])
					a := atoi(parts[4])
					R := atof(parts[5])
					b.eng.AttachStrategy(strategies.NewEmaAtr(f, s, a, R))
					b.reply(bot, chatID, fmt.Sprintf("Стратегия EMA set: fast=%d slow=%d atr=%d R=%.2f", f, s, a, R))
				} else {
					b.reply(bot, chatID, "Формат: /set_strategy ema <fast> <slow> <atr> <R>")
				}
			default:
				b.reply(bot, chatID, "Неизвестная команда. Попробуй /status")
			}
		}
	}
}

func (b *Bot) status() string {
	return fmt.Sprintf("Mode: paper\nEquity: %.2f USD", b.eng.EquityUSD())
}

func (b *Bot) reply(bot *gobot.BotAPI, chatID int64, text string) {
	msg := gobot.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Error().Err(err).Msg("send tg msg")
	}
}

func atoi(s string) int     { var x int; fmt.Sscanf(s, "%d", &x); return x }
func atof(s string) float64 { var x float64; fmt.Sscanf(s, "%f", &x); return x }
