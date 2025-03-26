package storage

import (
	"context"
	"fmt"
	"neuro_scout_bot_v1/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

type SourcePostgresStorage struct {
	db *sqlx.DB
}

func NewSourceStorage(db *sqlx.DB) *SourcePostgresStorage {
	return &SourcePostgresStorage{db: db}
}

type SourcesParams struct {
	Limit  int
	Offset int
}

func DefaultSourcesParams() SourcesParams {
	return SourcesParams{
		Limit:  10,
		Offset: 0,
	}
}

func (s *SourcePostgresStorage) Sources(ctx context.Context) ([]model.Source, error) {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	var sources []dbSource
	if err := conn.SelectContext(ctx, &sources, "SELECT * FROM sources"); err != nil {
		return nil, fmt.Errorf("failed to select sources: %w", err)
	}

	result := make([]model.Source, 0, len(sources))
	for _, source := range sources {
		addedAt, err := time.Parse(time.RFC3339, source.CreatedAt)
		if err != nil {
			addedAt = time.Time{}
		}

		result = append(result, model.Source{
			ID:        source.ID,
			Name:      source.Name,
			FeedURL:   source.FeedURL,
			Priority:  source.Priority,
			CreatedAt: addedAt,
		})
	}

	return result, nil
}

func (s *SourcePostgresStorage) SourceById(ctx context.Context, id int64) (model.Source, error) {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return model.Source{}, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	var source dbSource
	if err := conn.GetContext(ctx, &source, "SELECT * FROM sources WHERE id = $1", id); err != nil {
		return model.Source{}, fmt.Errorf("failed to get source by id: %w", err)
	}

	addedAt, err := time.Parse(time.RFC3339, source.CreatedAt)
	if err != nil {
		addedAt = time.Time{}
	}

	return model.Source{
		ID:        source.ID,
		Name:      source.Name,
		FeedURL:   source.FeedURL,
		Priority:  source.Priority,
		CreatedAt: addedAt,
	}, nil
}

func (s *SourcePostgresStorage) SourceByID(ctx context.Context, id int64) (*model.Source, error) {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	var source dbSource
	if err := conn.GetContext(ctx, &source, "SELECT * FROM sources WHERE id = $1", id); err != nil {
		return nil, fmt.Errorf("failed to get source by id: %w", err)
	}

	addedAt, err := time.Parse(time.RFC3339, source.CreatedAt)
	if err != nil {
		addedAt = time.Time{}
	}

	return &model.Source{
		ID:        source.ID,
		Name:      source.Name,
		FeedURL:   source.FeedURL,
		Priority:  source.Priority,
		CreatedAt: addedAt,
	}, nil
}

func (s *SourcePostgresStorage) SetPriority(ctx context.Context, sourceID int64, priority int) error {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "UPDATE sources SET priority = $1 WHERE id = $2", priority, sourceID); err != nil {
		return fmt.Errorf("failed to update source priority: %w", err)
	}

	return nil
}

func (s *SourcePostgresStorage) Add(ctx context.Context, source model.Source) (int64, error) {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	var id int64
	err = conn.QueryRowxContext(
		ctx,
		"INSERT INTO sources (name, feed_url, priority, created_at) VALUES ($1, $2, $3, $4) RETURNING id",
		source.Name, source.FeedURL, source.Priority, source.CreatedAt,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to insert source: %w", err)
	}

	return id, nil
}

func (s *SourcePostgresStorage) Delete(ctx context.Context, id int64) error {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "DELETE FROM sources WHERE id = $1", id); err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	return nil
}

type dbSource struct {
	ID        int64  `db:"id"`
	Name      string `db:"name"`
	FeedURL   string `db:"feed_url"`
	Priority  int64  `db:"priority"`
	CreatedAt string `db:"created_at"`
}
