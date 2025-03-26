package bot

import (
	"context"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"neuro_scout_bot_v1/internal/botkit"
)

// OpenAIStatusChecker interface for checking OpenAI API status
type OpenAIStatusChecker interface {
	CheckAPIKeyStatus() (string, error)
}

// ViewCmdCheckOpenAI handles the command to check OpenAI API key status
func ViewCmdCheckOpenAI(checker OpenAIStatusChecker) botkit.ViewFunc {
	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		if update.Message == nil {
			return fmt.Errorf("empty message")
		}

		// Send message about starting the check
		checkingMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "🔍 Checking OpenAI API key status...")
		checkingMsgResult, err := bot.Send(checkingMsg)
		if err != nil {
			return fmt.Errorf("failed to send checking message: %w", err)
		}

		// Perform the check
		status, err := checker.CheckAPIKeyStatus()
		if err != nil {
			log.Printf("[ERROR] Failed to check OpenAI API key status: %v", err)
			// Error is not critical, just show the status
		}

		// Determine emoji based on status
		var emoji string
		if err == nil {
			emoji = "✅"
		} else {
			emoji = "⚠️"
		}

		// Edit message with the result
		resultMsg := tgbotapi.NewEditMessageText(
			update.Message.Chat.ID,
			checkingMsgResult.MessageID,
			fmt.Sprintf("%s <b>OpenAI API Key Status:</b>\n\n%s", emoji, status))
		resultMsg.ParseMode = "HTML"

		if _, err := bot.Send(resultMsg); err != nil {
			return fmt.Errorf("failed to send result message: %w", err)
		}

		// If there's an error, send additional information on how to fix the problem
		if err != nil {
			helpText := `<b>📋 Troubleshooting Instructions:</b>

1️⃣ If you see a quota exceeded error (429):
   • Check your account status on the OpenAI platform
   • You may need to add funds or change your plan

2️⃣ If you see an authorization error (401):
   • Verify your API key in config.local.hcl
   • Generate a new key on the OpenAI platform

3️⃣ In case of server errors (5xx):
   • These are temporary issues on OpenAI's side, try again later

<i>To update your API key, edit the config.local.hcl file and restart the bot.</i>`

			helpMsg := tgbotapi.NewMessage(update.Message.Chat.ID, helpText)
			helpMsg.ParseMode = "HTML"

			if _, err := bot.Send(helpMsg); err != nil {
				return fmt.Errorf("failed to send help message: %w", err)
			}
		}

		return nil
	}
}
