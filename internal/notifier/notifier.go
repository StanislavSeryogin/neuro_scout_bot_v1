package notifier

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

	"neuro_scout_bot_v1/internal/botkit/markup"
	"neuro_scout_bot_v1/internal/model"
)

type ArticleProvider interface {
	AllNotPosted(ctx context.Context, since time.Time, limit uint64) ([]model.Article, error)
	MarkAsPosted(ctx context.Context, article model.Article) error
	FindRecentUniqueTitles(ctx context.Context, title string, since time.Time) (bool, error)
	HighPriorityNotPosted(ctx context.Context, priorityThreshold int64, since time.Time, limit uint64) ([]model.Article, error)
}

type Summarizer interface {
	Summarize(text string) (string, error)
}

type Notifier struct {
	articles         ArticleProvider
	summarizer       Summarizer
	bot              *tgbotapi.BotAPI
	sendInterval     time.Duration
	lookupTimeWindow time.Duration
	channelID        int64
}

func New(
	articleProvider ArticleProvider,
	summarizer Summarizer,
	bot *tgbotapi.BotAPI,
	sendInterval time.Duration,
	lookupTimeWindow time.Duration,
	channelID int64,
) *Notifier {
	return &Notifier{
		articles:         articleProvider,
		summarizer:       summarizer,
		bot:              bot,
		sendInterval:     sendInterval,
		lookupTimeWindow: lookupTimeWindow,
		channelID:        channelID,
	}
}

func (n *Notifier) Start(ctx context.Context) error {
	ticker := time.NewTicker(n.sendInterval)
	defer ticker.Stop()

	if err := n.ProcessHighPriorityArticles(ctx); err != nil {
		return err
	}

	if err := n.SelectAndSendArticle(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ticker.C:
			if err := n.ProcessHighPriorityArticles(ctx); err != nil {
				return err
			}

			if err := n.SelectAndSendArticle(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (n *Notifier) ProcessHighPriorityArticles(ctx context.Context) error {
	const priorityThreshold = 8

	highPriorityArticles, err := n.articles.HighPriorityNotPosted(
		ctx,
		priorityThreshold,
		time.Now().Add(-n.lookupTimeWindow),
		10,
	)
	if err != nil {
		return fmt.Errorf("failed to get high priority articles: %w", err)
	}

	if len(highPriorityArticles) == 0 {
		return nil
	}

	log.Printf("[INFO] Found %d high priority articles for auto-publishing", len(highPriorityArticles))

	for _, article := range highPriorityArticles {
		isUnique, err := n.articles.FindRecentUniqueTitles(ctx, article.Title, time.Now().AddDate(0, 0, -7))
		if err != nil {
			log.Printf("[WARN] Failed to check title uniqueness for high priority article: %v", err)
			if err := n.articles.MarkAsPosted(ctx, article); err != nil {
				log.Printf("[ERROR] Failed to mark high priority article as posted: %v", err)
			}
			continue
		}

		if !isUnique {
			log.Printf("[INFO] Skipping non-unique high priority article: %s", article.Title)
			if err := n.articles.MarkAsPosted(ctx, article); err != nil {
				log.Printf("[ERROR] Failed to mark high priority article as posted: %v", err)
			}
			continue
		}

		if err := n.PublishArticle(ctx, article); err != nil {
			log.Printf("[ERROR] Failed to publish high priority article: %v", err)
			continue
		}

		log.Printf("[INFO] Successfully published high priority article: %s", article.Title)

		time.Sleep(5 * time.Second)
	}

	return nil
}

func (n *Notifier) SelectAndSendArticle(ctx context.Context) error {
	topOneArticles, err := n.articles.AllNotPosted(ctx, time.Now().Add(-n.lookupTimeWindow), 1)
	if err != nil {
		return err
	}

	if len(topOneArticles) == 0 {
		return nil
	}

	article := topOneArticles[0]

	isUnique, err := n.articles.FindRecentUniqueTitles(ctx, article.Title, time.Now().AddDate(0, 0, -7))
	if err != nil {
		log.Printf("[WARN] Failed to check title uniqueness: %v", err)
		return n.articles.MarkAsPosted(ctx, article)
	}

	if !isUnique {
		log.Printf("[INFO] Skipping non-unique article: %s", article.Title)
		return n.articles.MarkAsPosted(ctx, article)
	}

	summary, err := n.extractSummary(article)
	if err != nil {
		log.Printf("[ERROR] failed to extract summary: %v", err)
	}

	if err := n.sendArticle(article, summary); err != nil {
		return err
	}

	return n.articles.MarkAsPosted(ctx, article)
}

var redundantNewLines = regexp.MustCompile(`\n{3,}`)

func (n *Notifier) extractSummary(article model.Article) (string, error) {
	log.Printf("[INFO] Extracting summary for article: %s", article.Title)

	// Перевіряємо наявність summarizer
	if n.summarizer == nil {
		log.Printf("[INFO] Summarizer is not configured, skipping summary generation")
		return "", nil
	}

	var r io.Reader

	if article.Summary != "" {
		log.Printf("[INFO] Using existing article summary")
		r = strings.NewReader(article.Summary)
	} else {
		log.Printf("[INFO] Article has no summary, fetching content from URL: %s", article.Link)
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		req, err := http.NewRequest("GET", article.Link, nil)
		if err != nil {
			log.Printf("[ERROR] Failed to create request: %v", err)
			return "", err
		}

		// Додамо User-Agent щоб уникнути блокування
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

	log.Printf("[INFO] Parsing article content with readability")
	doc, err := readability.FromReader(r, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to parse article with readability: %v", err)
		return "", err
	}

	textContent := cleanupText(doc.TextContent)

	if len(textContent) < 100 {
		log.Printf("[WARN] Article text is too short (%d chars), may not generate good summary", len(textContent))
		if len(textContent) < 20 {
			return "", fmt.Errorf("article text is too short to summarize")
		}
	}

	contentPreview := textContent
	if len(contentPreview) > 100 {
		contentPreview = contentPreview[:100] + "..."
	}
	log.Printf("[INFO] Article content extracted, length: %d chars, preview: %s", len(textContent), contentPreview)

	log.Printf("[INFO] Sending to summarizer")
	summary, err := n.summarizer.Summarize(textContent)
	if err != nil {
		// Якщо помилка пов'язана з відсутністю API ключа, просто пропускаємо генерацію summary
		if strings.Contains(err.Error(), "disabled") || strings.Contains(err.Error(), "401") ||
			strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "quota exceeded") {
			log.Printf("[INFO] Skipping summary generation due to API limitations: %v", err)
			return "", nil
		}

		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "quota exceeded") {
			log.Printf("[ERROR] OpenAI API quota exceeded (429 Too Many Requests): %v", err)
		} else if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
			log.Printf("[ERROR] OpenAI API unauthorized - invalid API key: %v", err)
		} else if strings.Contains(err.Error(), "too short") {
			log.Printf("[ERROR] Article text is too short to summarize: %v", err)
		} else if strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "502") || strings.Contains(err.Error(), "503") {
			log.Printf("[ERROR] OpenAI API server error: %v", err)
		} else {
			log.Printf("[ERROR] Failed to generate summary: %v", err)
		}
		return "", err
	}

	log.Printf("[INFO] Summary generated successfully: %s", summary)
	return "\n\n" + summary, nil
}

func cleanupText(text string) string {
	return redundantNewLines.ReplaceAllString(text, "\n")
}

func (n *Notifier) sendArticle(article model.Article, summary string) error {
	// Перевіряємо, чи summary не є порожнім
	const msgFormatWithSummary = "*%s*%s\n\n%s"
	const msgFormatWithoutSummary = "*%s*\n\n%s"

	var formattedMsg string
	if summary != "" {
		formattedMsg = fmt.Sprintf(
			msgFormatWithSummary,
			markup.EscapeForMarkdown(article.Title),
			markup.EscapeForMarkdown(summary),
			markup.EscapeForMarkdown(article.Link),
		)
		log.Printf("[INFO] Sending article to channel with summary. Title: %s, Summary length: %d, Message length: %d",
			article.Title, len(summary), len(formattedMsg))
	} else {
		formattedMsg = fmt.Sprintf(
			msgFormatWithoutSummary,
			markup.EscapeForMarkdown(article.Title),
			markup.EscapeForMarkdown(article.Link),
		)
		log.Printf("[INFO] Sending article to channel without summary. Title: %s, Message length: %d",
			article.Title, len(formattedMsg))
	}

	msg := tgbotapi.NewMessage(n.channelID, formattedMsg)
	msg.ParseMode = "MarkdownV2"

	_, err := n.bot.Send(msg)
	if err != nil {
		log.Printf("[ERROR] Failed to send article to channel: %v", err)
		return err
	}

	log.Printf("[INFO] Successfully sent article to channel: %s", article.Title)
	return nil
}

func (n *Notifier) PublishArticle(ctx context.Context, article model.Article) error {
	summary, err := n.extractSummary(article)
	if err != nil {
		log.Printf("[WARN] Failed to extract summary for auto-published article: %v", err)
		summary = ""
	}

	if err := n.sendArticle(article, summary); err != nil {
		return err
	}

	return n.articles.MarkAsPosted(ctx, article)
}
