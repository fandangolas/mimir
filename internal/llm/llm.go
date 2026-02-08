package llm

import "context"

// Role represents the sender of a message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single entry in a conversation.
type Message struct {
	Role    Role
	Content string
}

// ChatProvider is the interface for LLM chat completions.
// Only Ollama is supported; the interface exists to keep the orchestrator
// decoupled from the Ollama client directly.
type ChatProvider interface {
	// Chat sends a list of messages and returns the assistant's reply.
	Chat(ctx context.Context, messages []Message) (string, error)

	// SupportsToolCalling verifies that the loaded model supports native tool
	// calling. Returns an error if the model is unsupported.
	SupportsToolCalling(ctx context.Context) error
}
