package bot

import (
	"context"
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"neuro_scout_bot_v1/internal/botkit"
	"neuro_scout_bot_v1/internal/botkit/markup"
	"neuro_scout_bot_v1/internal/model"
)

type SourceProvider interface {
	SourceByID(ctx context.Context, id int64) (*model.Source, error)
}

func ViewCmdGetSource(provider SourceProvider) botkit.ViewFunc {
	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		idStr := update.Message.CommandArguments()

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return err
		}

		source, err := provider.SourceByID(ctx, id)
		if err != nil {
			return err
		}

		reply := tgbotapi.NewMessage(update.Message.Chat.ID, formatSource(*source))
		reply.ParseMode = parseModeMarkdownV2

		if _, err := bot.Send(reply); err != nil {
			return err
		}

		return nil
	}
}

func formatSource(source model.Source) string {
	return fmt.Sprintf(
		"🌐 *%s*\nID: `%d`\nURL feed: %s\nPriority: %d",
		markup.EscapeForMarkdown(source.Name),
		source.ID,
		markup.EscapeForMarkdown(source.FeedURL),
		source.Priority,
	)
}
