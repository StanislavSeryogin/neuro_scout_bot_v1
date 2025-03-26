package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"neuro_scout_bot_v1/internal/botkit"
)

type PrioritySetter interface {
	SetPriority(ctx context.Context, sourceID int64, priority int) error
}

func ViewCmdSetPriority(prioritySetter PrioritySetter) botkit.ViewFunc {
	type setPriorityArgs struct {
		SourceID int64 `json:"source_id"`
		Priority int   `json:"priority"`
	}

	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		args, err := botkit.ParseJSON[setPriorityArgs](update.Message.CommandArguments())
		if err != nil {
			// Informative message about command format
			helpMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
				"❌ Incorrect command format. Example: <code>/setpriority {\"source_id\":1,\"priority\":5}</code>\n\n"+
					"Priority affects the sorting of articles and sources. Higher priority = earlier publication.")
			helpMsg.ParseMode = "HTML"
			if _, err := bot.Send(helpMsg); err != nil {
				return err
			}
			return err
		}

		processMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("⏳ Setting priority %d for source with ID %d...",
				args.Priority, args.SourceID))
		processMsgResult, err := bot.Send(processMsg)
		if err != nil {
			return err
		}

		if err := prioritySetter.SetPriority(ctx, args.SourceID, args.Priority); err != nil {
			errorMsg := tgbotapi.NewEditMessageText(
				update.Message.Chat.ID,
				processMsgResult.MessageID,
				fmt.Sprintf("❌ Error setting priority: %v", err))
			if _, err := bot.Send(errorMsg); err != nil {
				return err
			}
			return err
		}
		var priorityInfo string
		if args.Priority >= 8 {
			priorityInfo = "📢 Articles from this source will be automatically published to the channel (priority ≥ 8)"
		} else {
			priorityInfo = "ℹ️ Articles will be published in the general queue (priority ≥ 8 needed for auto-publishing)"
		}

		successMsg := tgbotapi.NewEditMessageText(
			update.Message.Chat.ID,
			processMsgResult.MessageID,
			fmt.Sprintf("✅ Priority successfully updated!\n\nSource ID: %d\nNew priority: %d\n\n%s",
				args.SourceID, args.Priority, priorityInfo))
		if _, err := bot.Send(successMsg); err != nil {
			return err
		}

		return nil
	}
}
