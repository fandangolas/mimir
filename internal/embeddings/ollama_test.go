package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaEmbedder_Embed(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		responseCode  int
		responseBody  embeddingResponse
		expectError   bool
		errorContains string
	}{
		{
			name: "successful embedding",
			text: "hello world",
			responseCode: http.StatusOK,
			responseBody: embeddingResponse{
				Embedding: []float32{0.1, 0.2, 0.3},
			},
			expectError: false,
		},
		{
			name:          "empty text",
			text:          "",
			expectError:   true,
			errorContains: "text cannot be empty",
		},
		{
			name:          "server error",
			text:          "hello",
			responseCode:  http.StatusInternalServerError,
			expectError:   true,
			errorContains: "ollama returned status 500",
		},
		{
			name:         "empty embedding response",
			text:         "hello",
			responseCode: http.StatusOK,
			responseBody: embeddingResponse{
				Embedding: []float32{},
			},
			expectError:   true,
			errorContains: "received empty embedding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.responseCode == 0 {
					tt.responseCode = http.StatusOK
				}

				w.WriteHeader(tt.responseCode)
				if tt.responseCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			embedder := NewOllamaEmbedder(server.URL, "test-model")
			ctx := context.Background()

			result, err := embedder.Embed(ctx, tt.text)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != len(tt.responseBody.Embedding) {
					t.Errorf("expected %d dimensions, got %d", len(tt.responseBody.Embedding), len(result))
				}
			}
		})
	}
}

func TestOllamaEmbedder_EmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(embeddingResponse{
			Embedding: []float32{0.1, 0.2, 0.3},
		})
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "test-model")
	ctx := context.Background()

	texts := []string{"hello", "world", "test"}
	results, err := embedder.EmbedBatch(ctx, texts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != len(texts) {
		t.Errorf("expected %d results, got %d", len(texts), len(results))
	}

	for i, result := range results {
		if len(result) != 3 {
			t.Errorf("result %d: expected 3 dimensions, got %d", i, len(result))
		}
	}
}

func TestOllamaEmbedder_DefaultModel(t *testing.T) {
	embedder := NewOllamaEmbedder("http://localhost:11434", "")

	if embedder.model != "nomic-embed-text" {
		t.Errorf("expected default model 'nomic-embed-text', got %q", embedder.model)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr)+1 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
