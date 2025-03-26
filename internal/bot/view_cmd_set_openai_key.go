package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"neuro_scout_bot_v1/internal/botkit"
	"neuro_scout_bot_v1/internal/config"
)

func ViewCmdSetOpenAIKey() botkit.ViewFunc {
	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		if update.Message == nil {
			return fmt.Errorf("empty message")
		}

		apiKey := strings.TrimSpace(update.Message.CommandArguments())

		if apiKey == "" {
			helpMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
				`<b>How to set OpenAI API key:</b>

Use the command in the format:
<code>/setopenaikey YOUR_API_KEY</code>

Where YOUR_API_KEY is your OpenAI API key, starting with "sk-".

<b>How to get an API key:</b>
1. Register at https://platform.openai.com
2. Go to API keys section
3. Click "Create new secret key"
4. Copy the generated key

<i>Note: The key will be stored locally and used only for generating article summaries.</i>`)
			helpMsg.ParseMode = "HTML"

			if _, err := bot.Send(helpMsg); err != nil {
				return fmt.Errorf("failed to send help message: %w", err)
			}

			return nil
		}

		if !strings.HasPrefix(apiKey, "sk-") {
			errorMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
				"❌ <b>Error:</b> The provided key has an incorrect format. OpenAI API key must start with 'sk-'.")
			errorMsg.ParseMode = "HTML"

			if _, err := bot.Send(errorMsg); err != nil {
				return fmt.Errorf("failed to send error message: %w", err)
			}

			return nil
		}

		config.UpdateOpenAIKey(apiKey)

		successMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
			"✅ <b>OpenAI API key successfully set!</b>\n\nNow the bot will use your key to generate article summaries.\n\nUse the /checkopenai command to check the key status.")
		successMsg.ParseMode = "HTML"

		if _, err := bot.Send(successMsg); err != nil {
			return fmt.Errorf("failed to send success message: %w", err)
		}

		log.Printf("[INFO] OpenAI API key has been updated by user %s", update.Message.From.UserName)
		return nil
	}
}
