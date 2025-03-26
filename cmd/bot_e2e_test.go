package main

import (
	"context"
	"database/sql"
	"neuro_scout_bot_v1/internal/model"
	"neuro_scout_bot_v1/internal/storage"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndBot demonstrates how to write end-to-end tests for the bot
// Note: This is meant to be run against a real database in a controlled environment
func TestEndToEndBot(t *testing.T) {
	// Skip in regular unit test runs unless specifically requested
	// To run e2e tests: go test -tags=e2e ./...
	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	// Get database connection string from environment variable
	dbConnString := os.Getenv("TEST_DB_CONNECTION")
	if dbConnString == "" {
		t.Skip("Skipping test due to missing TEST_DB_CONNECTION environment variable")
	}

	// Connect to the real database (this would be your test database)
	db, err := sql.Open("postgres", dbConnString)
	require.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")
	sourceStorage := storage.NewSourceStorage(sqlxDB)

	ctx := context.Background()

	// Clean up the database before the test
	cleanupDatabase(t, sqlxDB)

	// Test adding a source
	source := model.Source{
		Name:      "E2E Test Source",
		FeedURL:   "https://example.com/rss",
		Priority:  5,
		CreatedAt: time.Now().UTC(),
	}

	sourceID, err := sourceStorage.Add(ctx, source)
	require.NoError(t, err)
	assert.Greater(t, sourceID, int64(0))

	// Test retrieving the source
	retrievedSource, err := sourceStorage.SourceByID(ctx, sourceID)
	require.NoError(t, err)
	assert.Equal(t, source.Name, retrievedSource.Name)
	assert.Equal(t, source.FeedURL, retrievedSource.FeedURL)

	// Test updating the source priority
	newPriority := 10
	err = sourceStorage.SetPriority(ctx, sourceID, newPriority)
	require.NoError(t, err)

	updatedSource, err := sourceStorage.SourceByID(ctx, sourceID)
	require.NoError(t, err)
	assert.Equal(t, int64(newPriority), updatedSource.Priority)

	// Test deleting the source
	err = sourceStorage.Delete(ctx, sourceID)
	require.NoError(t, err)

	// Verify it's deleted
	_, err = sourceStorage.SourceByID(ctx, sourceID)
	require.Error(t, err) // Should return an error when source not found

	// Clean up after test
	cleanupDatabase(t, sqlxDB)
}

// Helper function to clean up the database before/after tests
func cleanupDatabase(t *testing.T, db *sqlx.DB) {
	_, err := db.Exec("DELETE FROM sources WHERE name = 'E2E Test Source'")
	require.NoError(t, err)
}
