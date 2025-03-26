package fetcher

import (
	"context"
	"fmt"
	"log"
	"neuro_scout_bot_v1/internal/model"
	sourcelib "neuro_scout_bot_v1/internal/source"
	"strings"
	"time"

	"go.tomakado.io/containers/set"
)

type ArticleStorage interface {
	Store(ctx context.Context, article model.Article) error
	FindRecentUniqueTitles(ctx context.Context, title string, since time.Time) (bool, error)
}

type SourceProvider interface {
	Sources(ctx context.Context) ([]model.Source, error)
}

type Source interface {
	ID() int64
	Name() string
	Fetch(ctx context.Context) ([]model.Item, error)
}

type Fetcher struct {
	articles ArticleStorage
	sources  SourceProvider

	fetchInterval  time.Duration
	filterKeywords []string
}

func New(articlesStorage ArticleStorage, sourcesProvider SourceProvider, fetchInterval time.Duration, filterKeywords []string) *Fetcher {
	return &Fetcher{
		articles:       articlesStorage,
		sources:        sourcesProvider,
		fetchInterval:  fetchInterval,
		filterKeywords: filterKeywords,
	}
}

func (f *Fetcher) Start(ctx context.Context) error {
	ticker := time.NewTicker(f.fetchInterval)
	defer ticker.Stop()

	if err := f.Fetch(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := f.Fetch(ctx); err != nil {
				return err
			}
		}
	}
}

func (f *Fetcher) Fetch(ctx context.Context) error {
	sources, err := f.sources.Sources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sources: %w", err)
	}

	log.Printf("[INFO] Starting to fetch from %d sources", len(sources))

	for i, source := range sources {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			log.Printf("[INFO] Fetching source %d/%d: %s (priority: %d)",
				i+1, len(sources), source.Name, source.Priority)

			if err := f.fetchSource(ctx, source); err != nil {
				log.Printf("[ERROR] failed to fetch source %q: %v", source.Name, err)
				continue
			}

			waitTime := 30 * time.Second
			log.Printf("[INFO] Waiting %v before fetching next source", waitTime)
			time.Sleep(waitTime)
		}
	}

	log.Printf("[INFO] Completed fetching from all sources")
	return nil
}

func (f *Fetcher) fetchSource(ctx context.Context, sourceModel model.Source) error {
	fetchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	rssSource := sourcelib.NewRSSSourceFromModel(sourceModel)

	items, err := rssSource.Fetch(fetchCtx)
	if err != nil {
		return fmt.Errorf("failed to fetch items from source %q: %w", rssSource.Name(), err)
	}

	log.Printf("[INFO] Fetched %d items from source %q", len(items), rssSource.Name())

	if err := f.processItems(ctx, rssSource, items); err != nil {
		return fmt.Errorf("failed to process items from source %q: %w", rssSource.Name(), err)
	}

	return nil
}

func (f *Fetcher) processItems(ctx context.Context, source Source, items []model.Item) error {
	for _, item := range items {
		item.Date = item.Date.UTC()

		if f.itemShouldBeSkipped(item) {
			log.Printf("[INFO] item %q (%s) from source %q should be skipped", item.Title, item.Link, source.Name())
			continue
		}

		// Check article uniqueness
		isUnique, err := f.articles.FindRecentUniqueTitles(ctx, item.Title, time.Now().AddDate(0, 0, -7))
		if err != nil {
			log.Printf("[WARN] Failed to check article uniqueness: %v", err)
		} else if !isUnique {
			log.Printf("[INFO] Skipping non-unique article: %s", item.Title)
			continue
		}

		article := model.Article{
			SourceID:    source.ID(),
			Title:       item.Title,
			Link:        item.Link,
			Summary:     item.Summary,
			PublishedAt: item.Date,
		}

		if err := f.articles.Store(ctx, article); err != nil {
			return err
		}
	}

	return nil
}

func (f *Fetcher) itemShouldBeSkipped(item model.Item) bool {
	categoriesSet := set.New(item.Categories...)

	for _, keyword := range f.filterKeywords {
		if categoriesSet.Contains(keyword) || strings.Contains(strings.ToLower(item.Title), keyword) {
			return true
		}
	}

	return false
}
