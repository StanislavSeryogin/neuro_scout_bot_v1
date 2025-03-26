package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ViewCmdStart(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
	if update.Message == nil {
		return fmt.Errorf("empty message")
	}

	welcomeText := `<b>👋 Welcome to Neuro Scout Bot!</b>

The bot collects articles from various RSS sources and publishes them to the channel.

<b>📋 Available commands:</b>

<b>Source management:</b>
• <code>/listsources</code> - view all sources
• <code>/getsource</code> <i>{id}</i> - get information about a source
• <code>/addsource</code> <i>{"name":"Name", "url":"URL", "priority":number}</i> - add a new source
• <code>/deletesource</code> <i>{"source_id":number}</i> - delete a source
• <code>/setpriority</code> <i>{"source_id":number, "priority":number}</i> - set source priority (>=8 for auto-publishing)

<b>Finding and publishing articles:</b>
• <code>/findarticles</code> <i>{"period":"week", "limit":10}</i> - find articles (period: day, week, month)
• <code>/publishtochannel</code> <i>{"period":"week", "limit":5}</i> - publish articles to the channel

<b>OpenAI settings (for summary generation):</b>
• <code>/setopenaikey</code> <i>your-api-key</i> - set OpenAI API key
• <code>/checkopenai</code> - check OpenAI API key status

<b>Priority</b> affects the order of source display and article publication. Sources with priority >=8 are automatically published to the channel.

<i>Click on a command to copy it to the input field.</i>`

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeText)
	msg.ParseMode = "HTML"

	if _, err := bot.Send(msg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	examplesText := `<b>🔍 Example commands for quick use:</b>

<code>/listsources</code>
<code>/findarticles {"period":"week","limit":5}</code>
<code>/findarticles {"period":"day","limit":10}</code>
<code>/publishtochannel {"period":"week","limit":3}</code>
<code>/addsource {"name":"Golang Blog","url":"https://go.dev/blog/feed.atom","priority":5}</code>
<code>/setpriority {"source_id":1,"priority":8}</code>

<i>Click on a command to copy it and use it immediately.</i>`

	examplesMsg := tgbotapi.NewMessage(update.Message.Chat.ID, examplesText)
	examplesMsg.ParseMode = "HTML"

	if _, err := bot.Send(examplesMsg); err != nil {
		return fmt.Errorf("failed to send examples message: %w", err)
	}

	menuText := "Quick commands:"
	menuMsg := tgbotapi.NewMessage(update.Message.Chat.ID, menuText)

	menuMsg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/listsources"),
			tgbotapi.NewKeyboardButton("/findarticles"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/addsource"),
			tgbotapi.NewKeyboardButton("/setpriority"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/publishtochannel"),
			tgbotapi.NewKeyboardButton("/checkopenai"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/setopenaikey"),
		),
	)

	if _, err := bot.Send(menuMsg); err != nil {
		return fmt.Errorf("failed to send keyboard menu: %w", err)
	}

	return nil
}
