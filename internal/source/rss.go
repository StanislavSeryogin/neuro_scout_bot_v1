package source

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"neuro_scout_bot_v1/internal/model"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

type RSSSource struct {
	URL        string
	SourceId   int64
	SourceName string
	Priority   int64
	client     *http.Client
}

func NewRSSSourceFromModel(m model.Source) *RSSSource {
	return &RSSSource{
		URL:        m.FeedURL,
		SourceId:   m.ID,
		SourceName: m.Name,
		Priority:   m.Priority,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *RSSSource) Fetch(ctx context.Context) ([]model.Item, error) {
	feed, err := s.loadFeedWithRetry(ctx, s.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to load feed from %s: %w", s.URL, err)
	}

	items := make([]model.Item, 0, len(feed.Items))
	for _, item := range feed.Items {
		var pubDate time.Time
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		}

		items = append(items, model.Item{
			Title:      item.Title,
			Categories: item.Categories,
			Link:       item.Link,
			Date:       pubDate,
			Summary:    item.Description,
			SourceName: s.SourceName,
		})
	}
	return items, nil
}

func (s *RSSSource) loadFeedWithRetry(ctx context.Context, url string) (*gofeed.Feed, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * 3 * time.Second
			log.Printf("[INFO] Retry %d for %s, waiting %v", attempt, url, backoff)
			time.Sleep(backoff)
		}

		feed, err := s.loadFeed(ctx, url)
		if err == nil {
			return feed, nil
		}

		if strings.Contains(err.Error(), "429") ||
			strings.Contains(err.Error(), "too many requests") ||
			strings.Contains(err.Error(), "rate limit") {
			log.Printf("[WARN] Rate limit detected for %s: %v", url, err)
			time.Sleep(time.Duration(15*(1<<uint(attempt))) * time.Second)
		}

		lastErr = err
	}
	return nil, lastErr
}

func (s *RSSSource) loadFeed(ctx context.Context, url string) (*gofeed.Feed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36")
	req.Header.Set("Accept", "application/rss+xml, application/xml, application/atom+xml, text/xml, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	var (
		feedCH = make(chan *gofeed.Feed)
		errCH  = make(chan error)
	)

	go func() {
		parser := gofeed.NewParser()
		parser.Client = s.client

		feed, err := parser.ParseURLWithContext(url, ctx)
		if err != nil {
			errCH <- err
			return
		}
		feedCH <- feed
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCH:
		return nil, err
	case feed := <-feedCH:
		return feed, nil
	}
}

func (s *RSSSource) ID() int64 {
	return s.SourceId
}

func (s *RSSSource) Name() string {
	return s.SourceName
}
