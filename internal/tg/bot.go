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
	tl    core.TradeLogger
}

func NewBot(token string, eng *core.Engine, tl core.TradeLogger) *Bot {
	return &Bot{token: token, eng: eng, tl: tl}
}

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
			case strings.HasPrefix(text, "/start") || strings.HasPrefix(text, "/help"):
				b.reply(bot, chatID, helpText())
			case strings.HasPrefix(text, "/status"):
				b.reply(bot, chatID, b.status())
			case strings.HasPrefix(text, "/equity"):
				b.reply(bot, chatID, b.equity())
			case strings.HasPrefix(text, "/start_trading"):
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
			case strings.HasPrefix(text, "/history"):
				n := 10
				parts := strings.Fields(text)
				if len(parts) >= 2 {
					if v, err := atoiMaybe(parts[1]); err == nil && v > 0 {
						n = v
					}
				}
				rows, err := b.tl.LastN(n)
				if err != nil {
					b.reply(bot, chatID, "Ошибка чтения журнала")
				} else {
					b.reply(bot, chatID, formatHistory(rows))
				}
			default:
				b.reply(bot, chatID, "Неизвестная команда. /help")
			}
		}
	}
}

func (b *Bot) status() string {
	s := b.eng.Snapshot()
	return fmt.Sprintf("Mode: paper\nEquity: %.2f USD\nPos: %s qty=%.4f entry=%.2f unrl=%.2f",
		s.EquityUSD, actName(s.Position.Side), s.Position.Qty, s.Position.Entry, s.Position.Unreal)
}

func (b *Bot) equity() string {
	s := b.eng.Snapshot()
	return fmt.Sprintf("Equity: %.2f USD | Pos: %s %.4f @ %.2f (unrl=%.2f)",
		s.EquityUSD, actName(s.Position.Side), s.Position.Qty, s.Position.Entry, s.Position.Unreal)
}

func helpText() string {
	return "" +
		"Команды:\n" +
		"/help — справка\n" +
		"/status — режим, equity, позиция\n" +
		"/equity — кратко equity/позиция\n" +
		"/start_trading — включить уведомления (демо)\n" +
		"/stop_trading — выключить уведомления (демо)\n" +
		"/set_strategy ema <fast> <slow> <atr> <R> — параметры EMA\n" +
		"/history [N] — последние N записей журнала (по умолчанию 10)"
}

func formatHistory(rows []core.TradeLogEntry) string {
	if len(rows) == 0 {
		return "Журнал пуст"
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s %s %s %s qty=%.4f px=%.2f pnl=%.2f %s\n",
			r.TS.Format("2006-01-02 15:04"), r.Symbol, r.TF, r.Event, r.Qty, r.Price, r.PnL, r.Comment)
	}
	return b.String()
}

func (b *Bot) reply(bot *gobot.BotAPI, chatID int64, text string) {
	msg := gobot.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Error().Err(err).Msg("send tg msg")
	}
}

func actName(a core.Action) string {
	switch a {
	case core.Buy:
		return "LONG"
	case core.Sell:
		return "SHORT"
	default:
		return "FLAT"
	}
}

func atoi(s string) int               { var x int; fmt.Sscanf(s, "%d", &x); return x }
func atof(s string) float64           { var x float64; fmt.Sscanf(s, "%f", &x); return x }
func atoiMaybe(s string) (int, error) { var x int; _, err := fmt.Sscanf(s, "%d", &x); return x, err }
