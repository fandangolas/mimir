package rag

import (
	"fmt"
	"strings"
	"time"

	"github.com/fandangolas/mimir/internal/store"
)

// ContextManager assembles context from retrieved and recent messages.
type ContextManager struct {
	maxTokens    int
	systemTokens int
	queryTokens  int
	replyTokens  int
}

// NewContextManager creates a new context manager.
func NewContextManager(maxTokens, systemTokens, queryTokens, replyTokens int) *ContextManager {
	return &ContextManager{
		maxTokens:    maxTokens,
		systemTokens: systemTokens,
		queryTokens:  queryTokens,
		replyTokens:  replyTokens,
	}
}

// BuildContext creates the final context string for the LLM.
func (cm *ContextManager) BuildContext(
	systemPrompt string,
	retrievedMessages []SearchResult,
	recentMessages []store.Message,
	currentQuery string,
) string {
	var b strings.Builder

	// System prompt
	b.WriteString(systemPrompt)
	b.WriteString("\n\n")

	// Deduplicate: remove retrieved messages that are in recent messages
	retrievedMessages = cm.deduplicateMessages(recentMessages, retrievedMessages)

	// Add retrieved context from RAG (if any)
	if len(retrievedMessages) > 0 {
		b.WriteString("## Relevant conversation history:\n\n")
		for _, msg := range retrievedMessages {
			b.WriteString(fmt.Sprintf("[%s] %s: %s\n",
				msg.CreatedAt.Format("Jan 02 15:04"),
				msg.Role,
				msg.Content))
		}
		b.WriteString("\n")
	}

	// Add recent conversation
	if len(recentMessages) > 0 {
		b.WriteString("## Recent conversation:\n\n")
		for _, msg := range recentMessages {
			b.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
		}
		b.WriteString("\n")
	}

	// Current query
	b.WriteString(fmt.Sprintf("User: %s\n", currentQuery))
	b.WriteString("Assistant: ")

	return b.String()
}

// deduplicateMessages removes retrieved messages that appear in recent messages.
func (cm *ContextManager) deduplicateMessages(
	recent []store.Message,
	retrieved []SearchResult,
) []SearchResult {
	// Build set of recent message IDs
	recentIDs := make(map[int64]bool)
	for _, msg := range recent {
		recentIDs[msg.ID] = true
	}

	// Filter out retrieved messages that are already in recent
	var deduplicated []SearchResult
	for _, msg := range retrieved {
		if !recentIDs[msg.MessageID] {
			deduplicated = append(deduplicated, msg)
		}
	}

	return deduplicated
}

// EstimateTokens roughly estimates token count (1 token ≈ 4 characters).
func (cm *ContextManager) EstimateTokens(text string) int {
	return len(text) / 4
}

// TruncateMessages truncates messages to fit within token budget.
func (cm *ContextManager) TruncateMessages(
	retrieved []SearchResult,
	recent []store.Message,
) ([]SearchResult, []store.Message) {
	// Calculate available tokens
	availableTokens := cm.maxTokens - cm.systemTokens - cm.queryTokens - cm.replyTokens

	// Allocate budget: 40% to retrieved, 60% to recent
	retrievedBudget := int(float64(availableTokens) * 0.4)
	recentBudget := int(float64(availableTokens) * 0.6)

	// Truncate retrieved messages
	retrievedTruncated := cm.truncateRetrievedMessages(retrieved, retrievedBudget)

	// Truncate recent messages (keep most recent)
	recentTruncated := cm.truncateRecentMessages(recent, recentBudget)

	return retrievedTruncated, recentTruncated
}

func (cm *ContextManager) truncateRetrievedMessages(
	messages []SearchResult,
	maxTokens int,
) []SearchResult {
	var result []SearchResult
	currentTokens := 0

	for _, msg := range messages {
		msgTokens := cm.EstimateTokens(msg.Content)
		if currentTokens+msgTokens > maxTokens {
			break
		}
		result = append(result, msg)
		currentTokens += msgTokens
	}

	return result
}

func (cm *ContextManager) truncateRecentMessages(
	messages []store.Message,
	maxTokens int,
) []store.Message {
	// Process in reverse (most recent first) to keep latest messages
	var result []store.Message
	currentTokens := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		msgTokens := cm.EstimateTokens(msg.Content)
		if currentTokens+msgTokens > maxTokens {
			break
		}
		result = append([]store.Message{msg}, result...) // Prepend to maintain order
		currentTokens += msgTokens
	}

	return result
}

// MessageToSearchResult converts a store.Message to SearchResult for testing.
func MessageToSearchResult(msg store.Message, score float64) SearchResult {
	return SearchResult{
		MessageID:  msg.ID,
		Content:    msg.Content,
		Role:       msg.Role,
		CreatedAt:  msg.CreatedAt,
		FinalScore: score,
	}
}

// CreateSearchResult creates a SearchResult for testing.
func CreateSearchResult(id int64, content, role string, createdAt time.Time, score float64) SearchResult {
	return SearchResult{
		MessageID:  id,
		Content:    content,
		Role:       role,
		CreatedAt:  createdAt,
		FinalScore: score,
	}
}
