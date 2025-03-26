package summary

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

type OpenAISummarizer struct {
	client  *openai.Client
	prompt  string
	model   string
	enabled bool
	mu      sync.Mutex
}

func NewOpenAISummarizer(apiKey, model, prompt string) *OpenAISummarizer {
	s := &OpenAISummarizer{
		client: openai.NewClient(apiKey),
		prompt: prompt,
		model:  model,
	}

	log.Printf("openai summarizer is enabled: %v", apiKey != "")

	if apiKey != "" {
		s.enabled = true
	}

	return s
}

func (s *OpenAISummarizer) Summarize(text string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		log.Printf("[ERROR] OpenAI summarizer is disabled - no API key provided")
		return "", fmt.Errorf("openai summarizer is disabled")
	}

	log.Printf("[INFO] Generating summary with OpenAI using model: %s", s.model)

	request := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: s.prompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: text,
			},
		},
		MaxTokens:   1024,
		Temperature: 1,
		TopP:        1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx, request)
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			log.Printf("[ERROR] OpenAI API quota exceeded (429 Too Many Requests): %v", err)
			return "", fmt.Errorf("OpenAI API quota exceeded (429 Too Many Requests): %v", err)
		} else if strings.Contains(err.Error(), "400") {
			log.Printf("[ERROR] OpenAI API bad request (400): %v", err)
			return "", fmt.Errorf("OpenAI API bad request: %v", err)
		} else if strings.Contains(err.Error(), "401") {
			log.Printf("[ERROR] OpenAI API unauthorized (401) - invalid API key: %v", err)
			return "", fmt.Errorf("OpenAI API unauthorized - invalid API key: %v", err)
		} else if strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "502") || strings.Contains(err.Error(), "503") {
			log.Printf("[ERROR] OpenAI API server error: %v", err)
			return "", fmt.Errorf("OpenAI API server error: %v", err)
		} else {
			log.Printf("[ERROR] OpenAI API error: %v", err)
			return "", err
		}
	}

	if len(resp.Choices) == 0 {
		log.Printf("[ERROR] No choices in OpenAI response")
		return "", errors.New("no choices in openai response")
	}

	rawSummary := strings.TrimSpace(resp.Choices[0].Message.Content)
	log.Printf("[INFO] Successfully generated summary with OpenAI: %s", rawSummary[:min(80, len(rawSummary))])

	if strings.HasSuffix(rawSummary, ".") {
		return rawSummary, nil
	}

	// cut all after the last ".":
	sentences := strings.Split(rawSummary, ".")

	return strings.Join(sentences[:len(sentences)-1], ".") + ".", nil
}

// Додаємо допоміжну функцію min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CheckAPIKeyStatus checks the API key status by attempting to make the smallest possible request
func (s *OpenAISummarizer) CheckAPIKeyStatus() (string, error) {
	if !s.enabled {
		return "API key not configured", fmt.Errorf("openai summarizer is disabled")
	}

	log.Printf("[INFO] Checking OpenAI API key status")

	// Create the smallest possible request for checking
	request := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful assistant.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Hello",
			},
		},
		MaxTokens:   5, // Minimum number of tokens
		Temperature: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := s.client.CreateChatCompletion(ctx, request)
	if err != nil {
		// Process specific error types
		if strings.Contains(err.Error(), "429") {
			return "Error 429: API quota limit exceeded. Please check your subscription plan or add funds.", err
		} else if strings.Contains(err.Error(), "401") {
			return "Error 401: invalid API key. Check the key correctness.", err
		} else if strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "502") || strings.Contains(err.Error(), "503") {
			return "OpenAI server error. Try again later.", err
		} else {
			return fmt.Sprintf("Error checking API key: %v", err), err
		}
	}

	return "API key is working correctly", nil
}
