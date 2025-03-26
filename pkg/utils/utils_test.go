package utils

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetTimestamp(t *testing.T) {
	timestamp := GetTimestamp()

	// Test that the timestamp contains the expected format
	assert.True(t, strings.Contains(timestamp, "Current time:"))

	// Get the current time to compare
	currentTime := time.Now().Format(time.RFC3339)

	// The timestamp should contain the current time or be very close to it
	// We only check that the date portion matches since the exact seconds might differ
	datePart := currentTime[:10] // YYYY-MM-DD part
	assert.True(t, strings.Contains(timestamp, datePart),
		"Expected timestamp to contain today's date: %s, but got: %s", datePart, timestamp)
}
