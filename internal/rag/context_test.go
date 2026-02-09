package rag

import (
	"strings"
	"testing"
	"time"

	"github.com/fandangolas/mimir/internal/store"
)

func TestContextManager_BuildContext(t *testing.T) {
	cm := NewContextManager(4096, 200, 500, 1000)

	systemPrompt := "You are a helpful assistant."

	retrieved := []SearchResult{
		CreateSearchResult(1, "I like pizza", "user", time.Now().Add(-24*time.Hour), 0.9),
		CreateSearchResult(2, "Pizza is great!", "assistant", time.Now().Add(-24*time.Hour), 0.9),
	}

	recent := []store.Message{
		{ID: 10, Role: "user", Content: "What's for dinner?", CreatedAt: time.Now()},
		{ID: 11, Role: "assistant", Content: "How about pizza?", CreatedAt: time.Now()},
	}

	currentQuery := "Do I like Italian food?"

	context := cm.BuildContext(systemPrompt, retrieved, recent, currentQuery)

	// Verify context contains all components
	if !strings.Contains(context, systemPrompt) {
		t.Error("context missing system prompt")
	}

	if !strings.Contains(context, "Relevant conversation history") {
		t.Error("context missing retrieved section header")
	}

	if !strings.Contains(context, "I like pizza") {
		t.Error("context missing retrieved message")
	}

	if !strings.Contains(context, "Recent conversation") {
		t.Error("context missing recent section header")
	}

	if !strings.Contains(context, "What's for dinner?") {
		t.Error("context missing recent message")
	}

	if !strings.Contains(context, currentQuery) {
		t.Error("context missing current query")
	}
}

func TestContextManager_Deduplication(t *testing.T) {
	cm := NewContextManager(4096, 200, 500, 1000)

	recent := []store.Message{
		{ID: 5, Role: "user", Content: "Hello", CreatedAt: time.Now()},
		{ID: 6, Role: "assistant", Content: "Hi there", CreatedAt: time.Now()},
	}

	retrieved := []SearchResult{
		CreateSearchResult(5, "Hello", "user", time.Now(), 0.9),  // Duplicate with recent
		CreateSearchResult(10, "Old message", "user", time.Now().Add(-24*time.Hour), 0.8),
	}

	deduplicated := cm.deduplicateMessages(recent, retrieved)

	if len(deduplicated) != 1 {
		t.Errorf("expected 1 message after dedup, got %d", len(deduplicated))
	}

	if deduplicated[0].MessageID != 10 {
		t.Errorf("expected message ID 10, got %d", deduplicated[0].MessageID)
	}
}

func TestContextManager_EstimateTokens(t *testing.T) {
	cm := NewContextManager(4096, 200, 500, 1000)

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "short text",
			text:     "Hello", // 5 chars = ~1 token
			expected: 1,
		},
		{
			name:     "longer text",
			text:     "This is a longer sentence with many words", // 42 chars = ~10 tokens
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cm.EstimateTokens(tt.text)
			if result != tt.expected {
				t.Errorf("expected %d tokens, got %d", tt.expected, result)
			}
		})
	}
}

func TestContextManager_TruncateMessages(t *testing.T) {
	cm := NewContextManager(1000, 200, 100, 200) // Very limited budget

	// Create messages that exceed budget
	retrieved := []SearchResult{
		CreateSearchResult(1, strings.Repeat("a", 400), "user", time.Now(), 0.9),
		CreateSearchResult(2, strings.Repeat("b", 400), "user", time.Now(), 0.8),
		CreateSearchResult(3, strings.Repeat("c", 400), "user", time.Now(), 0.7),
	}

	recent := []store.Message{
		{ID: 10, Role: "user", Content: strings.Repeat("x", 400), CreatedAt: time.Now()},
		{ID: 11, Role: "assistant", Content: strings.Repeat("y", 400), CreatedAt: time.Now()},
		{ID: 12, Role: "user", Content: strings.Repeat("z", 400), CreatedAt: time.Now()},
	}

	retrievedTrunc, recentTrunc := cm.TruncateMessages(retrieved, recent)

	// Verify truncation happened
	if len(retrievedTrunc) >= len(retrieved) && len(recentTrunc) >= len(recent) {
		t.Error("expected truncation to reduce message count")
	}

	// Verify we didn't keep more messages than budget allows
	// Available: 1000 - 200 - 100 - 200 = 500 tokens
	// Retrieved budget: 500 * 0.4 = 200 tokens
	// Recent budget: 500 * 0.6 = 300 tokens

	totalRetrievedChars := 0
	for _, msg := range retrievedTrunc {
		totalRetrievedChars += len(msg.Content)
	}

	totalRecentChars := 0
	for _, msg := range recentTrunc {
		totalRecentChars += len(msg.Content)
	}

	// Each budget should be roughly respected (with some tolerance)
	if totalRetrievedChars > 1000 { // 200 tokens * 4 chars/token + tolerance
		t.Errorf("retrieved messages exceed budget: %d chars", totalRetrievedChars)
	}

	if totalRecentChars > 1500 { // 300 tokens * 4 chars/token + tolerance
		t.Errorf("recent messages exceed budget: %d chars", totalRecentChars)
	}
}

func TestContextManager_TruncateRecentMessages_PreservesLatest(t *testing.T) {
	cm := NewContextManager(4096, 200, 500, 1000)

	messages := []store.Message{
		{ID: 1, Role: "user", Content: "Old message 1", CreatedAt: time.Now().Add(-3 * time.Hour)},
		{ID: 2, Role: "user", Content: "Old message 2", CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: 3, Role: "user", Content: "Recent message 1", CreatedAt: time.Now().Add(-1 * time.Hour)},
		{ID: 4, Role: "user", Content: "Recent message 2", CreatedAt: time.Now()},
	}

	// Very small budget to force truncation
	truncated := cm.truncateRecentMessages(messages, 50) // ~12 tokens

	if len(truncated) == 0 {
		t.Fatal("truncated to zero messages")
	}

	// Should keep most recent messages
	lastMessage := truncated[len(truncated)-1]
	if lastMessage.ID != 4 {
		t.Errorf("expected to keep most recent message (ID 4), got ID %d", lastMessage.ID)
	}
}
