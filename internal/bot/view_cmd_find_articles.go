package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"neuro_scout_bot_v1/internal/botkit"
	"neuro_scout_bot_v1/internal/botkit/markup"
	"neuro_scout_bot_v1/internal/model"
)

type ArticleFinder interface {
	FindArticlesByTimePeriod(ctx context.Context, since time.Time, limit uint64) ([]model.Article, error)
}

func ViewCmdFindArticles(finder ArticleFinder) botkit.ViewFunc {
	type findArticlesArgs struct {
		Period string `json:"period"` // "day", "week", "month"
		Limit  int    `json:"limit"`
	}

	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		args, err := botkit.ParseJSON[findArticlesArgs](update.Message.CommandArguments())
		if err != nil {

			helpMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
				"❌ Incorrect command format. Example: <code>/findarticles {\"period\":\"week\",\"limit\":10}</code>\n\n"+
					"Supported periods: <code>day</code>, <code>week</code>, <code>month</code>\n"+
					"Limit: from 1 to 50 articles")
			helpMsg.ParseMode = "HTML"
			if _, err := bot.Send(helpMsg); err != nil {
				return err
			}
			return err
		}

		limit := uint64(10)
		if args.Limit > 0 && args.Limit <= 50 {
			limit = uint64(args.Limit)
		}

		var since time.Time
		now := time.Now()

		searchingMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("🔍 Searching for articles for period: %s, limit: %d...",
				strings.ToLower(args.Period), limit))
		searchingMsgResult, err := bot.Send(searchingMsg)
		if err != nil {
			return err
		}

		periodName := "week"
		switch strings.ToLower(args.Period) {
		case "day":
			since = now.AddDate(0, 0, -1)
			periodName = "day"
		case "week":
			since = now.AddDate(0, 0, -7)
			periodName = "week"
		case "month":
			since = now.AddDate(0, -1, 0)
			periodName = "month"
		default:

			since = now.AddDate(0, 0, -7)
		}

		articles, err := finder.FindArticlesByTimePeriod(ctx, since, limit)
		if err != nil {

			errorMsg := tgbotapi.NewEditMessageText(
				update.Message.Chat.ID,
				searchingMsgResult.MessageID,
				fmt.Sprintf("❌ Error searching for articles: %v", err))
			if _, err := bot.Send(errorMsg); err != nil {
				return err
			}
			return err
		}

		if len(articles) == 0 {

			noResultsMsg := tgbotapi.NewEditMessageText(
				update.Message.Chat.ID,
				searchingMsgResult.MessageID,
				"ℹ️ No articles found for the specified period")
			if _, err := bot.Send(noResultsMsg); err != nil {
				return err
			}
			return nil
		}

		foundMsg := tgbotapi.NewEditMessageText(
			update.Message.Chat.ID,
			searchingMsgResult.MessageID,
			fmt.Sprintf("✅ Found %d articles for period: %s", len(articles), periodName))
		if _, err := bot.Send(foundMsg); err != nil {
			return err
		}

		var articlesFormatted []string
		for _, article := range articles {
			pubDate := article.PublishedAt.Format("2006-01-02 15:04")
			articleStr := fmt.Sprintf("📌 *%s*\n⏰ %s\n🔗 %s",
				markup.EscapeForMarkdown(article.Title),
				markup.EscapeForMarkdown(pubDate),
				markup.EscapeForMarkdown(article.Link))
			articlesFormatted = append(articlesFormatted, articleStr)
		}

		msgText := fmt.Sprintf(
			"Found articles for period: *%s* \\(%d articles\\)\n\n%s",
			markup.EscapeForMarkdown(periodName),
			len(articles),
			strings.Join(articlesFormatted, "\n\n"),
		)

		if len(msgText) > 4000 {
			chunks := splitLongMessage(msgText, 4000)
			for _, chunk := range chunks {
				reply := tgbotapi.NewMessage(update.Message.Chat.ID, chunk)
				reply.ParseMode = parseModeMarkdownV2
				if _, err := bot.Send(reply); err != nil {
					return err
				}
			}
		} else {
			reply := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
			reply.ParseMode = parseModeMarkdownV2
			if _, err := bot.Send(reply); err != nil {
				return err
			}
		}

		return nil
	}
}

func splitLongMessage(text string, maxLength int) []string {
	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLength {
			chunks = append(chunks, text)
			break
		}

		cutPoint := maxLength
		for i := maxLength; i >= 0; i-- {
			if text[i] == '\n' {
				cutPoint = i + 1
				break
			}
		}

		chunks = append(chunks, text[:cutPoint])
		text = text[cutPoint:]
	}
	return chunks
}
