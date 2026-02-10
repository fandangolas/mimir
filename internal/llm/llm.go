package llm

import (
	"context"
	"encoding/json"
)

// Role represents the sender of a message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single entry in a conversation.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // For tool response messages
}

// Tool represents a function the LLM can call.
type Tool struct {
	Type     string       `json:"type"` // Always "function"
	Function FunctionDef  `json:"function"`
}

// FunctionDef describes a callable function.
type FunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON Schema object
}

// ToolCall represents the LLM's decision to call a tool.
type ToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"` // Always "function"
	Function FunctionCall        `json:"function"`
}

// FunctionCall contains the function name and arguments.
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"` // JSON object or string
}

// ChatProvider is the interface for LLM chat completions.
// Only Ollama is supported; the interface exists to keep the orchestrator
// decoupled from the Ollama client directly.
type ChatProvider interface {
	// Chat sends a list of messages and returns the assistant's reply.
	Chat(ctx context.Context, messages []Message) (string, error)

	// ChatWithTools sends messages with available tools and returns the response.
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*Message, error)

	// SupportsToolCalling verifies that the loaded model supports native tool
	// calling. Returns an error if the model is unsupported.
	SupportsToolCalling(ctx context.Context) error
}
