package ollama

import (
	"context"
	"testing"
)

func TestSupportsToolCalling_knownModel(t *testing.T) {
	tests := []struct {
		model   string
		wantErr bool
	}{
		{"llama3.1", false},
		{"llama3.1:8b", false},
		{"qwen2.5", false},
		{"qwen2.5:14b", false},
		{"llama3.2", false},
		{"gpt-4", true},
		{"mistral", true},
		{"unknown-model", true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			c := New("http://localhost:11434", tt.model)
			err := c.SupportsToolCalling(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("model %q: wantErr=%v, got err=%v", tt.model, tt.wantErr, err)
			}
		})
	}
}
