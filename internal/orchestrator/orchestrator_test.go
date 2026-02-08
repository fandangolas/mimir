package orchestrator

import (
	"context"
	"testing"

	"github.com/fandangolas/personal-assistant/internal/llm"
	"github.com/fandangolas/personal-assistant/internal/store"
)

// mockChat is a test double for llm.ChatProvider.
type mockChat struct {
	reply string
	err   error
	calls [][]llm.Message
}

func (m *mockChat) Chat(_ context.Context, messages []llm.Message) (string, error) {
	m.calls = append(m.calls, messages)
	return m.reply, m.err
}

func (m *mockChat) SupportsToolCalling(_ context.Context) error { return nil }

func TestBuildMessages_includesSystemPrompt(t *testing.T) {
	history := []store.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	msgs := buildMessages(history)

	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (system + 2 history), got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("first message should be system, got %q", msgs[0].Role)
	}
	if msgs[1].Content != "hello" {
		t.Errorf("expected user message content 'hello', got %q", msgs[1].Content)
	}
}

func TestBuildMessages_emptyHistory(t *testing.T) {
	msgs := buildMessages(nil)

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (system only), got %d", len(msgs))
	}
	if msgs[0].Role != llm.RoleSystem {
		t.Errorf("expected system role, got %q", msgs[0].Role)
	}
}
