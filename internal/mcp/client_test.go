package mcp

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("echo", nil, nil)
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.pending == nil {
		t.Error("expected pending map to be initialized")
	}

	if client.nextID.Load() != 0 {
		t.Error("expected nextID to start at 0")
	}
}

func TestClientConfigure(t *testing.T) {
	client := NewClient("echo", nil, nil)

	command := "npx"
	args := []string{"@cocal/google-calendar-mcp"}
	env := []string{"GOOGLE_OAUTH_CREDENTIALS=/path/to/creds"}

	client.Configure(command, args, env)

	if client.cmd == nil {
		t.Error("expected cmd to be configured")
	}
}

func TestFormatToolResult(t *testing.T) {
	client := NewClient("echo", nil, nil)

	tests := []struct {
		name     string
		result   *ToolResult
		expected string
	}{
		{
			name: "single text content",
			result: &ToolResult{
				Content: []Content{
					{Type: "text", Text: "Hello World"},
				},
			},
			expected: "Hello World",
		},
		{
			name: "multiple text contents",
			result: &ToolResult{
				Content: []Content{
					{Type: "text", Text: "Hello "},
					{Type: "text", Text: "World"},
				},
			},
			expected: "Hello World",
		},
		{
			name:     "empty content",
			result:   &ToolResult{Content: []Content{}},
			expected: "",
		},
		{
			name: "non-text content",
			result: &ToolResult{
				Content: []Content{
					{Type: "image", Text: "ignored"},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.formatToolResult(tt.result)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
