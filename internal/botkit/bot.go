package botkit

import (
	"context"
	"encoding/json"
	"log"
	"runtime/debug"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api      *tgbotapi.BotAPI
	cmdViews map[string]ViewFunc
}

func New(api *tgbotapi.BotAPI) *Bot {
	return &Bot{
		api:      api,
		cmdViews: make(map[string]ViewFunc),
	}
}

func (b *Bot) RegisterCmdView(cmd string, view ViewFunc) {
	if b.cmdViews == nil {
		b.cmdViews = make(map[string]ViewFunc)
	}

	b.cmdViews[cmd] = view
}

func (b *Bot) GetCmdView(cmd string) (ViewFunc, bool) {
	view, ok := b.cmdViews[cmd]
	return view, ok
}

func (b *Bot) Run(ctx context.Context) error {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	var lastUpdateID int
	consecutiveErrorCount := 0
	errorBackoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if lastUpdateID != 0 {
				updateConfig.Offset = lastUpdateID + 1
			}

			updates, err := b.api.GetUpdates(updateConfig)
			if err != nil {
				consecutiveErrorCount++
				log.Printf("[ERROR] Failed to get updates (attempt %d): %v", consecutiveErrorCount, err)

				if consecutiveErrorCount > 1 {
					errorBackoff = time.Duration(min(int64(errorBackoff)*2, int64(30*time.Second)))
				}

				if consecutiveErrorCount > 3 {
					log.Printf("[INFO] Resetting update configuration after multiple failures")
					updateConfig = tgbotapi.NewUpdate(0)
					updateConfig.Timeout = 30
					time.Sleep(5 * time.Second)
				} else {
					time.Sleep(errorBackoff)
				}
				continue
			}

			consecutiveErrorCount = 0
			errorBackoff = time.Second

			for _, update := range updates {
				if update.UpdateID > lastUpdateID {
					lastUpdateID = update.UpdateID
				}

				updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Minute)
				b.handleUpdate(updateCtx, update)
				updateCancel()
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("[ERROR] panic recovered: %v\n%s", p, string(debug.Stack()))
		}
	}()

	if (update.Message == nil || !update.Message.IsCommand()) && update.CallbackQuery == nil {
		return
	}

	var view ViewFunc

	if !update.Message.IsCommand() {
		return
	}

	cmd := update.Message.Command()

	cmdView, ok := b.cmdViews[cmd]
	if !ok {
		return
	}

	view = cmdView

	if err := view(ctx, b.api, update); err != nil {
		log.Printf("[ERROR] failed to execute view: %v", err)

		if _, err := b.api.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Internal error")); err != nil {
			log.Printf("[ERROR] failed to send error message: %v", err)
		}
	}
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

type ViewFunc func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error

func ParseJSON[T any](args string) (T, error) {
	var result T
	if err := json.Unmarshal([]byte(args), &result); err != nil {
		return result, err
	}
	return result, nil
}
