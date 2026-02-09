package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Embedder is the interface for generating text embeddings.
type Embedder interface {
	// Embed generates an embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts efficiently.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// OllamaEmbedder generates embeddings using Ollama's embedding API.
type OllamaEmbedder struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaEmbedder creates a new Ollama embedder.
// If model is empty, defaults to "nomic-embed-text".
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if model == "" {
		model = "nomic-embed-text"
	}

	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed generates an embedding for a single text.
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	reqBody := embeddingRequest{
		Model:  e.model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		fmt.Sprintf("%s/api/embeddings", e.baseURL),
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var embResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(embResp.Embedding) == 0 {
		return nil, fmt.Errorf("received empty embedding from ollama")
	}

	return embResp.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
// Processes sequentially to avoid overwhelming Ollama.
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}

		embeddings[i] = emb
	}

	return embeddings, nil
}
