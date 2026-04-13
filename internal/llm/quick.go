package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type quickResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

type openAIQuickResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// QuickComplete runs a non-streaming lightweight LLM call for tasks like
// query reformulation. Tries Anthropic Haiku first, falls back to the
// OpenAI-compatible endpoint, and returns ("", nil) silently when neither
// is available so callers can fall back gracefully.
func QuickComplete(ctx context.Context, system, user string) (string, error) {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return quickCompleteAnthropic(ctx, system, user, key)
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return quickCompleteOpenAI(ctx, system, user, key)
	}
	return "", nil
}

func quickCompleteAnthropic(ctx context.Context, system, user, key string) (string, error) {
	reqBody := struct {
		Model     string             `json:"model"`
		MaxTokens int                `json:"max_tokens"`
		System    string             `json:"system"`
		Messages  []anthropicMessage `json:"messages"`
	}{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 300,
		System:    system,
		Messages: []anthropicMessage{
			{Role: "user", Content: []contentBlock{{Type: "text", Text: user}}},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", nil
		}
		return "", fmt.Errorf("haiku request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("haiku API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var qr quickResponse
	if err := json.Unmarshal(respBody, &qr); err != nil {
		return "", err
	}
	if len(qr.Content) == 0 {
		return "", nil
	}
	return strings.TrimSpace(qr.Content[0].Text), nil
}

func quickCompleteOpenAI(ctx context.Context, system, user, key string) (string, error) {
	payload := openAIRequest{
		Model:     ResolveOpenAIModel(""),
		MaxTokens: 300,
		Messages: []openAIMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	endpoint := openAIBaseURL() + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", nil
		}
		return "", fmt.Errorf("openai quick request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var qr openAIQuickResponse
	if err := json.Unmarshal(respBody, &qr); err != nil {
		return "", err
	}
	if len(qr.Choices) == 0 || qr.Choices[0].Message.Content == "" {
		return "", nil
	}
	return strings.TrimSpace(qr.Choices[0].Message.Content), nil
}

