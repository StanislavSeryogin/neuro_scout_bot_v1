package storage

import (
	"context"
	"errors"
	"neuro_scout_bot_v1/internal/model"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourcePostgresStorage_Add(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	storage := NewSourceStorage(sqlxDB)

	ctx := context.Background()
	currentTime := time.Now().UTC()

	source := model.Source{
		Name:      "Test Source",
		FeedURL:   "https://test.com/feed",
		Priority:  10,
		CreatedAt: currentTime,
	}

	// Setup the expected query and response
	mock.ExpectQuery("INSERT INTO sources").
		WithArgs(source.Name, source.FeedURL, source.Priority, source.CreatedAt).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// Execute the method
	id, err := storage.Add(ctx, source)

	// Assert expectations
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSourcePostgresStorage_Sources(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	storage := NewSourceStorage(sqlxDB)

	ctx := context.Background()
	timeStr := time.Now().UTC().Format(time.RFC3339)

	// Setup the expected query and response
	rows := sqlmock.NewRows([]string{"id", "name", "feed_url", "priority", "created_at"}).
		AddRow(1, "Source 1", "https://source1.com/feed", 10, timeStr).
		AddRow(2, "Source 2", "https://source2.com/feed", 20, timeStr)

	mock.ExpectQuery("SELECT (.+) FROM sources").WillReturnRows(rows)

	// Execute the method
	sources, err := storage.Sources(ctx)

	// Assert expectations
	require.NoError(t, err)
	assert.Len(t, sources, 2)
	assert.Equal(t, "Source 1", sources[0].Name)
	assert.Equal(t, "Source 2", sources[1].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSourcePostgresStorage_SourceByID(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	storage := NewSourceStorage(sqlxDB)

	ctx := context.Background()
	timeStr := time.Now().UTC().Format(time.RFC3339)
	sourceID := int64(1)

	// Setup the expected query and response
	rows := sqlmock.NewRows([]string{"id", "name", "feed_url", "priority", "created_at"}).
		AddRow(sourceID, "Test Source", "https://test.com/feed", 10, timeStr)

	mock.ExpectQuery("SELECT (.+) FROM sources WHERE id = ?").
		WithArgs(sourceID).
		WillReturnRows(rows)

	// Execute the method
	source, err := storage.SourceByID(ctx, sourceID)

	// Assert expectations
	require.NoError(t, err)
	assert.NotNil(t, source)
	assert.Equal(t, sourceID, source.ID)
	assert.Equal(t, "Test Source", source.Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSourcePostgresStorage_Delete(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	storage := NewSourceStorage(sqlxDB)

	ctx := context.Background()
	sourceID := int64(1)

	// Setup the expected query
	mock.ExpectExec("DELETE FROM sources WHERE id = ?").
		WithArgs(sourceID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute the method
	err = storage.Delete(ctx, sourceID)

	// Assert expectations
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSourcePostgresStorage_SetPriority(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	storage := NewSourceStorage(sqlxDB)

	ctx := context.Background()
	sourceID := int64(1)
	newPriority := 20

	// Setup the expected query with PostgreSQL parameter style
	mock.ExpectExec("UPDATE sources SET priority = \\$1 WHERE id = \\$2").
		WithArgs(newPriority, sourceID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute the method
	err = storage.SetPriority(ctx, sourceID, newPriority)

	// Assert expectations
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSourcePostgresStorage_Error(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	storage := NewSourceStorage(sqlxDB)

	ctx := context.Background()
	sourceID := int64(1)

	// Setup the expected query with an error
	mock.ExpectQuery("SELECT (.+) FROM sources WHERE id = ?").
		WithArgs(sourceID).
		WillReturnError(errors.New("database error"))

	// Execute the method
	_, err = storage.SourceByID(ctx, sourceID)

	// Assert expectations
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get source by id")
	assert.NoError(t, mock.ExpectationsWereMet())
}
