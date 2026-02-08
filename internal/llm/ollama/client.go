package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"

	"github.com/fandangolas/mimir/internal/llm"
)

// toolCapableModels lists model name prefixes known to support native tool calling.
// Add more as needed when switching models.
var toolCapableModels = []string{
	"llama3.1",
	"llama3.2",
	"llama3.3",
	"qwen2.5",
	"qwen3",
	"mistral-nemo",
	"firefunction",
}

// Client is an Ollama-backed implementation of llm.ChatProvider.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
	breaker    *gobreaker.CircuitBreaker[string]
}

// New creates a new Ollama client with a circuit breaker.
func New(baseURL, model string) *Client {
	cb := gobreaker.NewCircuitBreaker[string](gobreaker.Settings{
		Name:        "ollama",
		MaxRequests: 1,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	})

	return &Client{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		breaker: cb,
	}
}

// chatRequest is the JSON body sent to the Ollama /api/chat endpoint.
type chatRequest struct {
	Model    string       `json:"model"`
	Messages []ollamaMsg  `json:"messages"`
	Stream   bool         `json:"stream"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Message ollamaMsg `json:"message"`
	Error   string    `json:"error,omitempty"`
}

// Chat sends messages to Ollama and returns the assistant reply.
// Calls are protected by a circuit breaker.
func (c *Client) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	result, err := c.breaker.Execute(func() (string, error) {
		return c.doChat(ctx, messages)
	})
	if err != nil {
		return "", fmt.Errorf("ollama chat: %w", err)
	}
	return result, nil
}

func (c *Client) doChat(ctx context.Context, messages []llm.Message) (string, error) {
	msgs := make([]ollamaMsg, len(messages))
	for i, m := range messages {
		msgs[i] = ollamaMsg{Role: string(m.Role), Content: m.Content}
	}

	body, err := json.Marshal(chatRequest{
		Model:    c.model,
		Messages: msgs,
		Stream:   false,
	})
	if err != nil {
		return "", fmt.Errorf("marshalling chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("decoding ollama response: %w", err)
	}

	if cr.Error != "" {
		return "", fmt.Errorf("ollama error: %s", cr.Error)
	}

	return cr.Message.Content, nil
}

// SupportsToolCalling checks whether the configured model is known to support
// native tool calling. Returns an error if it does not.
func (c *Client) SupportsToolCalling(ctx context.Context) error {
	for _, prefix := range toolCapableModels {
		if len(c.model) >= len(prefix) && c.model[:len(prefix)] == prefix {
			return nil
		}
	}
	return fmt.Errorf(
		"model %q does not support native tool calling; supported prefixes: %v",
		c.model, toolCapableModels,
	)
}

// showResponse is used to verify the model exists in Ollama.
type showRequest struct {
	Model string `json:"model"`
}

type showResponse struct {
	Error string `json:"error,omitempty"`
}

// Ping verifies that Ollama is reachable and the model is available.
func (c *Client) Ping(ctx context.Context) error {
	body, _ := json.Marshal(showRequest{Model: c.model})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/show", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating ping request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reaching ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("model %q not found in ollama; run: ollama pull %s", c.model, c.model)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama ping returned status %d", resp.StatusCode)
	}

	var sr showResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return fmt.Errorf("decoding ollama show response: %w", err)
	}
	if sr.Error != "" {
		return fmt.Errorf("ollama: %s", sr.Error)
	}

	return nil
}
