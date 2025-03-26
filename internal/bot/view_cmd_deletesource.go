package bot

import (
	"context"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"neuro_scout_bot_v1/internal/botkit"
)

type SourceDeleter interface {
	Delete(ctx context.Context, sourceID int64) error
}

func ViewCmdDeleteSource(deleter SourceDeleter) botkit.ViewFunc {
	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		idStr := update.Message.CommandArguments()

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return err
		}

		if err := deleter.Delete(ctx, id); err != nil {
			return nil
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "The source has been successfully deleted")
		if _, err := bot.Send(msg); err != nil {
			return err
		}

		return nil
	}
}
