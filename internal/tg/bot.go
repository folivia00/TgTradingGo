package tg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tradebot/internal/core"
	"tradebot/internal/state"
	"tradebot/internal/strategies"
)

type Bot struct {
	token      string
	eng        *core.Engine
	tl         core.TradeLogger
	store      *state.Store
	updateID   int64
	symbol     string
	tf         string
	feedType   string
	strategy   state.StrategyState
	switchFeed func(string)
}

func NewBot(token string, eng *core.Engine, tl core.TradeLogger, store *state.Store, symbol, tf, feedType string) *Bot {
	b := &Bot{token: token, eng: eng, tl: tl, store: store, symbol: symbol, tf: tf, feedType: feedType}
	b.captureStrategy(eng.Strategy())
	return b
}

func (b *Bot) Run(ctx context.Context, onSwitchFeed func(newFeed string)) error {
	b.switchFeed = onSwitchFeed
	if b.token == "" {
		log.Printf("TG token empty: bot disabled")
		return nil
	}
	if _, err := b.me(); err != nil {
		return fmt.Errorf("telegram connect: %w", err)
	}
	log.Printf("Telegram connected (HTTP)")

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			updates, err := b.getUpdates()
			if err != nil {
				log.Printf("getUpdates: %v", err)
				continue
			}
			for _, up := range updates {
				if up.Message == nil {
					continue
				}
				chatID := up.Message.Chat.ID
				text := strings.TrimSpace(up.Message.Text)
				switch {
				case strings.HasPrefix(text, "/start"), strings.HasPrefix(text, "/help"):
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
					if b.tl == nil {
						b.send(chatID, "Журнал недоступен")
						continue
					}
					rows, err := b.tl.LastN(n)
					if err != nil {
						b.send(chatID, "Ошибка чтения журнала")
					} else {
						b.send(chatID, formatHistory(rows))
					}
				case strings.HasPrefix(text, "/save_state"):
					if err := b.saveState(); err != nil {
						b.send(chatID, "Не удалось сохранить state")
					} else {
						b.send(chatID, "State сохранён")
					}
				case strings.HasPrefix(text, "/load_state"):
					if err := b.loadState(); err != nil {
						b.send(chatID, "Не удалось загрузить state")
					} else {
						b.send(chatID, "State загружен")
					}
				case strings.HasPrefix(text, "/reset_state"):
					if b.store == nil {
						b.send(chatID, "State store не настроен")
						break
					}
					if err := b.store.Reset(); err != nil {
						b.send(chatID, "Не удалось сбросить state")
					} else {
						b.send(chatID, "State сброшен")
					}
				case strings.HasPrefix(text, "/switch_feed"):
					parts := strings.Fields(text)
					if len(parts) < 2 {
						b.send(chatID, "Формат: /switch_feed rest|random")
						break
					}
					ft := parts[1]
					if ft != "rest" && ft != "random" {
						b.send(chatID, "Только rest|random")
						break
					}
					if ft != b.feedType {
						b.feedType = ft
						if b.switchFeed != nil {
							b.switchFeed(ft)
						}
					}
					b.send(chatID, "Фид переключён: "+ft)
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
		st := state.StrategyState{
			Type: "ema",
			I:    []int{atoi(parts[2]), atoi(parts[3]), atoi(parts[4])},
			F:    []float64{atof(parts[5])},
		}
		if err := b.applyStrategy(st); err != nil {
			b.send(chatID, "Не удалось применить стратегию")
			return
		}
		b.send(chatID, fmt.Sprintf("Стратегия EMA set: fast=%d slow=%d atr=%d R=%.2f", st.I[0], st.I[1], st.I[2], st.F[0]))
	case "rsi":
		if len(parts) < 6 {
			b.send(chatID, "Формат: /set_strategy rsi <len> <overbought> <oversold> <R>")
			return
		}
		st := state.StrategyState{
			Type: "rsi",
			I:    []int{atoi(parts[2])},
			F:    []float64{atof(parts[3]), atof(parts[4]), atof(parts[5])},
		}
		if err := b.applyStrategy(st); err != nil {
			b.send(chatID, "Не удалось применить стратегию")
			return
		}
		b.send(chatID, fmt.Sprintf("Стратегия RSI set: len=%d over=%.1f under=%.1f R=%.2f", st.I[0], st.F[0], st.F[1], st.F[2]))
	default:
		b.send(chatID, "Неизвестная стратегия. Используй: ema | rsi")
	}
}

func (b *Bot) saveState() error {
	if b.store == nil {
		return errors.New("state store nil")
	}
	b.captureStrategy(b.eng.Strategy())
	st := state.State{
		Strategy: b.strategy,
		Feed: state.FeedState{
			Type:   b.feedType,
			Symbol: b.symbol,
			TF:     b.tf,
		},
	}
	return b.store.Save(st)
}

func (b *Bot) loadState() error {
	if b.store == nil {
		return errors.New("state store nil")
	}
	st, err := b.store.Load()
	if err != nil {
		return err
	}
	if st.Strategy.Type != "" {
		if err := b.applyStrategy(st.Strategy); err != nil {
			return err
		}
	}
	prevFeed := b.feedType
	if st.Feed.Type == "rest" || st.Feed.Type == "random" {
		b.feedType = st.Feed.Type
		if st.Feed.Symbol != "" {
			b.symbol = st.Feed.Symbol
		}
		if st.Feed.TF != "" {
			b.tf = st.Feed.TF
		}
		if b.switchFeed != nil && prevFeed != b.feedType {
			b.switchFeed(b.feedType)
		}
	}
	return nil
}

func (b *Bot) applyStrategy(st state.StrategyState) error {
	switch st.Type {
	case "ema":
		if len(st.I) < 3 || len(st.F) < 1 {
			return errors.New("invalid ema params")
		}
		strat := strategies.NewEmaAtr(st.I[0], st.I[1], st.I[2], st.F[0])
		b.eng.AttachStrategy(strat)
		b.captureStrategy(strat)
	case "rsi":
		if len(st.I) < 1 || len(st.F) < 3 {
			return errors.New("invalid rsi params")
		}
		strat := strategies.NewRSI(st.I[0], st.F[0], st.F[1], st.F[2])
		b.eng.AttachStrategy(strat)
		b.captureStrategy(strat)
	default:
		return fmt.Errorf("unknown strategy: %s", st.Type)
	}
	return nil
}

func (b *Bot) captureStrategy(strat core.Strategy) {
	if strat == nil {
		b.strategy = state.StrategyState{}
		return
	}
	switch s := strat.(type) {
	case *strategies.EmaAtr:
		b.strategy = state.StrategyState{Type: "ema", I: []int{s.Fast, s.Slow, s.AtrLen}, F: []float64{s.RiskR}}
	case *strategies.RSI:
		b.strategy = state.StrategyState{Type: "rsi", I: []int{s.Len}, F: []float64{s.Overbought, s.Oversold, s.RiskR}}
	default:
		b.strategy = state.StrategyState{Type: strat.Name()}
	}
}

type tgUser struct {
	ID int64 `json:"id"`
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
		log.Printf("telegram send failed: %v", err)
		return
	}
	if !out.Ok {
		log.Printf("telegram send response not ok")
	}
}

func (b *Bot) status() string {
	s := b.eng.Snapshot()
	return fmt.Sprintf("Mode: paper\nFeed: %s\nEquity: %.2f USD\nPos: %s qty=%.4f entry=%.2f unrl=%.2f", b.feedType, s.EquityUSD, actName(s.Position.Side), s.Position.Qty, s.Position.Entry, s.Position.Unreal)
}

func (b *Bot) equity() string {
	s := b.eng.Snapshot()
	return fmt.Sprintf("Equity: %.2f USD | Pos: %s %.4f @ %.2f (unrl=%.2f)", s.EquityUSD, actName(s.Position.Side), s.Position.Qty, s.Position.Entry, s.Position.Unreal)
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
		"/switch_feed rest|random — переключить источник свечей\n" +
		"/save_state, /load_state, /reset_state — управление состоянием\n" +
		"/history [N] — последние N записей журнала (по умолчанию 10)"
}

func formatHistory(rows []core.TradeLogEntry) string {
	if len(rows) == 0 {
		return "Журнал пуст"
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s %s %s %s qty=%.4f px=%.2f pnl=%.2f %s\n", r.TS.Format("2006-01-02 15:04"), r.Symbol, r.TF, r.Event, r.Qty, r.Price, r.PnL, r.Comment)
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

func atoi(s string) int {
	var x int
	fmt.Sscanf(s, "%d", &x)
	return x
}

func atof(s string) float64 {
	var x float64
	fmt.Sscanf(s, "%f", &x)
	return x
}

func atoiMaybe(s string) (int, error) {
	var x int
	_, err := fmt.Sscanf(s, "%d", &x)
	return x, err
}
