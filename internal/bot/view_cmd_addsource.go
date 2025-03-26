package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"neuro_scout_bot_v1/internal/botkit"
	"neuro_scout_bot_v1/internal/model"
)

type SourceStorage interface {
	Add(ctx context.Context, source model.Source) (int64, error)
}

func ViewCmdAddSource(storage SourceStorage) botkit.ViewFunc {
	type addSourceArgs struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Priority int    `json:"priority"`
	}

	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		args, err := botkit.ParseJSON[addSourceArgs](update.Message.CommandArguments())
		if err != nil {
			helpMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
				"❌ Incorrect command format. Example: <code>/addsource {\"name\":\"Name\",\"url\":\"URL\",\"priority\":5}</code>\n\n"+
					"Required parameters:\n"+
					"- <code>name</code> - source name\n"+
					"- <code>url</code> - RSS/Atom feed URL\n"+
					"- <code>priority</code> - priority from 1 to 10")
			helpMsg.ParseMode = "HTML"
			if _, err := bot.Send(helpMsg); err != nil {
				return err
			}
			return err
		}

		source := model.Source{
			Name:     args.Name,
			FeedURL:  args.URL,
			Priority: int64(args.Priority),
		}

		sourceID, err := storage.Add(ctx, source)
		if err != nil {
			// TODO: send error message
			return err
		}

		var (
			msgText = fmt.Sprintf(
				"Source added with ID: `%d`\\. Use this ID to update or delete the source\\.",
				sourceID,
			)
			reply = tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
		)

		reply.ParseMode = parseModeMarkdownV2

		if _, err := bot.Send(reply); err != nil {
			return err
		}

		return nil
	}
}
