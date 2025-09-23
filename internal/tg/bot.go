package tg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"tradebot/internal/core"
	"tradebot/internal/strategies"
)

type Bot struct {
	token    string
	eng      *core.Engine
	tl       core.TradeLogger
	updateID int64
}

func NewBot(token string, eng *core.Engine, tl core.TradeLogger) *Bot {
	return &Bot{token: token, eng: eng, tl: tl}
}

func (b *Bot) Run(ctx context.Context) error {
	if b.token == "" {
		log.Warn().Msg("TG token empty: bot disabled")
		return nil
	}
	if _, err := b.me(); err != nil {
		return fmt.Errorf("telegram connect: %w", err)
	}
	log.Info().Msg("Telegram connected (HTTP)")

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			updates, err := b.getUpdates()
			if err != nil {
				log.Error().Err(err).Msg("getUpdates")
				continue
			}
			for _, up := range updates {
				if up.Message == nil {
					continue
				}
				chatID := up.Message.Chat.ID
				text := strings.TrimSpace(up.Message.Text)
				switch {
				case strings.HasPrefix(text, "/start") || strings.HasPrefix(text, "/help"):
					b.send(chatID, helpText())
				case strings.HasPrefix(text, "/status"):
					b.send(chatID, b.status())
				case strings.HasPrefix(text, "/equity"):
					b.send(chatID, b.equity())
				case strings.HasPrefix(text, "/which_strategy"):
					b.send(chatID, b.which())
				case strings.HasPrefix(text, "/start_trading"):
					b.send(chatID, "Торговля включена (paper)")
				case strings.HasPrefix(text, "/stop_trading"):
					b.send(chatID, "Торговля выключена")
				case strings.HasPrefix(text, "/set_strategy"):
					b.handleSetStrategy(chatID, text)
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
						b.send(chatID, "Ошибка чтения журнала")
					} else {
						b.send(chatID, formatHistory(rows))
					}
				default:
					b.send(chatID, "Неизвестная команда. /help")
				}
			}
		}
	}
}

func (b *Bot) handleSetStrategy(chatID int64, text string) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		b.send(chatID, "Формат: /set_strategy ema|rsi ...")
		return
	}
	switch parts[1] {
	case "ema":
		if len(parts) < 6 {
			b.send(chatID, "Формат: /set_strategy ema <fast> <slow> <atr> <R>")
			return
		}
		f := atoi(parts[2])
		s := atoi(parts[3])
		a := atoi(parts[4])
		R := atof(parts[5])
		b.eng.AttachStrategy(strategies.NewEmaAtr(f, s, a, R))
		b.send(chatID, fmt.Sprintf("Стратегия EMA set: fast=%d slow=%d atr=%d R=%.2f", f, s, a, R))
	case "rsi":
		if len(parts) < 6 {
			b.send(chatID, "Формат: /set_strategy rsi <len> <overbought> <oversold> <R>")
			return
		}
		l := atoi(parts[2])
		ob := atof(parts[3])
		os := atof(parts[4])
		R := atof(parts[5])
		b.eng.AttachStrategy(strategies.NewRSI(l, ob, os, R))
		b.send(chatID, fmt.Sprintf("Стратегия RSI set: len=%d over=%.1f under=%.1f R=%.2f", l, ob, os, R))
	default:
		b.send(chatID, "Неизвестная стратегия. Используй: ema | rsi")
	}
}

func (b *Bot) which() string {
	strat := b.eng.Strategy()
	if strat == nil {
		return "Стратегия не установлена"
	}
	switch s := strat.(type) {
	case *strategies.EmaAtr:
		return fmt.Sprintf("Активная стратегия: EMA_ATR fast=%d slow=%d atr=%d R=%.2f", s.Fast, s.Slow, s.AtrLen, s.RiskR)
	case *strategies.RSI:
		return fmt.Sprintf("Активная стратегия: RSI len=%d overbought=%.2f oversold=%.2f R=%.2f", s.Len, s.Overbought, s.Oversold, s.RiskR)
	default:
		return fmt.Sprintf("Активная стратегия: %s", strat.Name())
	}
}

// ==== Telegram HTTP client helpers ====

type tgUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

type tgMessage struct {
	MessageID int64  `json:"message_id"`
	Chat      tgChat `json:"chat"`
	Text      string `json:"text"`
}

type tgUpdate struct {
	UpdateID int64      `json:"update_id"`
	Message  *tgMessage `json:"message"`
}

type tgResp[T any] struct {
	Ok     bool `json:"ok"`
	Result T    `json:"result"`
}

func (b *Bot) api(method string, params url.Values, out any) error {
	u := "https://api.telegram.org/bot" + b.token + "/" + method
	resp, err := http.PostForm(u, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

func (b *Bot) me() (tgUser, error) {
	var resp tgResp[tgUser]
	if err := b.api("getMe", url.Values{}, &resp); err != nil {
		return tgUser{}, err
	}
	if !resp.Ok {
		return tgUser{}, fmt.Errorf("getMe not ok")
	}
	return resp.Result, nil
}

func (b *Bot) getUpdates() ([]tgUpdate, error) {
	v := url.Values{}
	if b.updateID != 0 {
		v.Set("offset", strconv.FormatInt(b.updateID+1, 10))
	}
	v.Set("timeout", "0")
	var resp tgResp[[]tgUpdate]
	if err := b.api("getUpdates", v, &resp); err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("getUpdates not ok")
	}
	if len(resp.Result) > 0 {
		b.updateID = resp.Result[len(resp.Result)-1].UpdateID
	}
	return resp.Result, nil
}

func (b *Bot) send(chatID int64, text string) {
	v := url.Values{}
	v.Set("chat_id", strconv.FormatInt(chatID, 10))
	v.Set("text", text)
	var out tgResp[tgMessage]
	if err := b.api("sendMessage", v, &out); err != nil {
		log.Error().Err(err).Msg("telegram send failed")
		return
	}
	if !out.Ok {
		log.Warn().Msg("telegram send response not ok")
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
		"/which_strategy — показать активную стратегию\n" +
		"/start_trading — включить уведомления (демо)\n" +
		"/stop_trading — выключить уведомления (демо)\n" +
		"/set_strategy ema <fast> <slow> <atr> <R>\n" +
		"/set_strategy rsi <len> <overbought> <oversold> <R>\n" +
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
