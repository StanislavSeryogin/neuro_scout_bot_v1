package bot

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"neuro_scout_bot_v1/internal/botkit"
	"neuro_scout_bot_v1/internal/botkit/markup"
	"neuro_scout_bot_v1/internal/model"
)

type ArticlePublisher interface {
	FindArticlesByTimePeriod(ctx context.Context, since time.Time, limit uint64) ([]model.Article, error)
	MarkAsPosted(ctx context.Context, article model.Article) error
}

type Summarizer interface {
	Summarize(text string) (string, error)
}

var redundantNewLines = regexp.MustCompile(`\n{3,}`)

func cleanupText(text string) string {
	return redundantNewLines.ReplaceAllString(text, "\n")
}

func extractSummary(summarizer Summarizer, article model.Article) (string, error) {
	var r io.Reader

	if article.Summary != "" {
		log.Printf("[INFO] Using existing article summary for: %s", article.Title)
		r = strings.NewReader(article.Summary)
	} else {
		log.Printf("[INFO] Fetching content from URL: %s", article.Link)
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		req, err := http.NewRequest("GET", article.Link, nil)
		if err != nil {
			log.Printf("[ERROR] Failed to create request: %v", err)
			return "", err
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[ERROR] Failed to fetch article from %s: %v", article.Link, err)
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("[ERROR] Bad response status: %s for URL: %s", resp.Status, article.Link)
			return "", fmt.Errorf("bad response status: %s", resp.Status)
		}

		r = resp.Body
	}

	log.Printf("[INFO] Parsing article with readability")
	doc, err := readability.FromReader(r, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to parse article with readability: %v", err)
		return "", err
	}

	text := cleanupText(doc.TextContent)

	if len(text) < 100 {
		log.Printf("[WARN] Article text is too short (%d chars), may not generate good summary", len(text))
		if len(text) < 20 {
			return "", fmt.Errorf("article text is too short to summarize")
		}
	}

	preview := text
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}
	log.Printf("[INFO] Article text extracted, length: %d chars, preview: %s", len(text), preview)

	if summarizer == nil {
		log.Printf("[ERROR] Summarizer is nil")
		return "", fmt.Errorf("summarizer is nil")
	}

	log.Printf("[INFO] Sending to summarizer: %s", article.Title)
	summary, err := summarizer.Summarize(text)
	if err != nil {
		log.Printf("[ERROR] Failed to generate summary: %v", err)
		return "", err
	}

	log.Printf("[INFO] Summary generated: %s", summary)
	return "\n\n" + summary, nil
}

func ViewCmdPublishToChannel(publisher ArticlePublisher, channelID int64, summarizer Summarizer) botkit.ViewFunc {
	type publishArgs struct {
		Period string `json:"period"`
		Limit  int    `json:"limit"`
	}

	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		args, err := botkit.ParseJSON[publishArgs](update.Message.CommandArguments())
		if err != nil {
			helpMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
				"❌ Incorrect command format. Example: <code>/publishtochannel {\"period\":\"week\",\"limit\":5}</code>\n\n"+
					"Supported periods: <code>day</code>, <code>week</code>, <code>month</code>\n"+
					"Limit: from 1 to 20 articles")
			helpMsg.ParseMode = "HTML"
			if _, err := bot.Send(helpMsg); err != nil {
				return err
			}
			return err
		}

		limit := uint64(5)
		if args.Limit > 0 && args.Limit <= 20 {
			limit = uint64(args.Limit)
		}

		searchingMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("🔍 Searching for articles for period: %s, limit: %d for publication to channel...",
				strings.ToLower(args.Period), limit))
		searchingMsgResult, err := bot.Send(searchingMsg)
		if err != nil {
			return err
		}

		var since time.Time
		now := time.Now()
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

		articles, err := publisher.FindArticlesByTimePeriod(ctx, since, limit)
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

		startMsg := tgbotapi.NewEditMessageText(
			update.Message.Chat.ID,
			searchingMsgResult.MessageID,
			fmt.Sprintf("✅ Found %d articles for period: %s\n⏳ Starting publication to channel...",
				len(articles), periodName))
		if _, err := bot.Send(startMsg); err != nil {
			return err
		}

		publishedCount := 0
		skippedCount := 0
		errorsCount := 0

		for i, article := range articles {
			var summary string
			if summarizer != nil {
				progressMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
					fmt.Sprintf("📝 Generating description for article %d/%d: %s", i+1, len(articles), article.Title))
				progressMsgResult, err := bot.Send(progressMsg)
				if err != nil {
					log.Printf("[ERROR] Failed to send progress message: %v", err)
				}

				summary, err = extractSummary(summarizer, article)
				if err != nil {
					log.Printf("[ERROR] Failed to extract summary: %v", err)
					summary = ""

					var errorMessage string
					if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota exceeded") {
						errorMessage = fmt.Sprintf("⚠️ Failed to generate description for article %d/%d: API quota exceeded. "+
							"Please check your OpenAI subscription or use a different API key.",
							i+1, len(articles))
					} else if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
						errorMessage = fmt.Sprintf("⚠️ Failed to generate description for article %d/%d: invalid API key.",
							i+1, len(articles))
					} else if strings.Contains(err.Error(), "too short") {
						errorMessage = fmt.Sprintf("⚠️ Failed to generate description for article %d/%d: article text is too short.",
							i+1, len(articles))
					} else if strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "502") || strings.Contains(err.Error(), "503") {
						errorMessage = fmt.Sprintf("⚠️ Failed to generate description for article %d/%d: OpenAI server error.",
							i+1, len(articles))
					} else {
						errorMessage = fmt.Sprintf("⚠️ Failed to generate description for article %d/%d: %v",
							i+1, len(articles), err)
					}

					if progressMsgResult.MessageID != 0 {
						errorMsg := tgbotapi.NewEditMessageText(
							update.Message.Chat.ID,
							progressMsgResult.MessageID,
							errorMessage)
						if _, err := bot.Send(errorMsg); err != nil {
							log.Printf("[ERROR] Failed to send summary error message: %v", err)
						}
					}
				} else {
					if progressMsgResult.MessageID != 0 {
						successMsg := tgbotapi.NewEditMessageText(
							update.Message.Chat.ID,
							progressMsgResult.MessageID,
							fmt.Sprintf("✅ Description for article %d/%d generated successfully", i+1, len(articles)))
						if _, err := bot.Send(successMsg); err != nil {
							log.Printf("[ERROR] Failed to send summary success message: %v", err)
						}
					}
				}
			}

			title := strings.ToValidUTF8(article.Title, "")
			link := strings.ToValidUTF8(article.Link, "")

			log.Printf("[INFO] Preparing message for article: %s, Summary exists: %v, Summary length: %d",
				title, summary != "", len(summary))

			const msgFormatWithSummary = "*%s*%s\n\n%s"
			const msgFormatWithoutSummary = "*%s*\n\n%s"

			var msgText string
			if summary != "" {
				msgText = fmt.Sprintf(
					msgFormatWithSummary,
					markup.EscapeForMarkdown(title),
					markup.EscapeForMarkdown(summary),
					markup.EscapeForMarkdown(link),
				)
				log.Printf("[INFO] Using format with summary")
			} else {
				msgText = fmt.Sprintf(
					msgFormatWithoutSummary,
					markup.EscapeForMarkdown(title),
					markup.EscapeForMarkdown(link),
				)
				log.Printf("[INFO] Using format without summary")
			}

			channelMsg := tgbotapi.NewMessage(channelID, msgText)
			channelMsg.ParseMode = "MarkdownV2"

			log.Printf("[INFO] Sending article %d/%d to channel: %s", i+1, len(articles), article.Title)

			_, err := bot.Send(channelMsg)
			if err != nil {
				errMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
					fmt.Sprintf("❌ Error publishing article %d: %v", i+1, err))
				if _, errSend := bot.Send(errMsg); errSend != nil {
					return errSend
				}
				errorsCount++
				continue
			}

			publishedCount++

			if err := publisher.MarkAsPosted(ctx, article); err != nil {
				fmt.Printf("Failed to mark article as posted: %v\n", err)
				skippedCount++
			}

			if (i+1)%3 == 0 && i > 0 {
				progressMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
					fmt.Sprintf("🔄 Published %d/%d articles", i+1, len(articles)))
				if _, err := bot.Send(progressMsg); err != nil {
					return err
				}
			}

			time.Sleep(3 * time.Second)
		}

		doneMsg := tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("✅ Article publication completed:\n"+
				"• Published: %d\n"+
				"• Errors: %d\n"+
				"• Not marked as published: %d\n\n"+
				"Check the channel to view published articles.",
				publishedCount, errorsCount, skippedCount))
		if _, err := bot.Send(doneMsg); err != nil {
			return err
		}

		return nil
	}
}
