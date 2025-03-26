package utils

import (
	"fmt"
	"time"
)

func GetTimestamp() string {
	return fmt.Sprintf("Current time: %s", time.Now().Format(time.RFC3339))
}
