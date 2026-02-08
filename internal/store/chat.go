package store

import (
	"context"
	"fmt"
	"time"
)

// Message represents a single chat message.
type Message struct {
	ID        int64
	SessionID int64
	Role      string
	Content   string
	CreatedAt time.Time
}

// GetOrCreateSession returns the session ID for the given Telegram chat ID,
// creating one if it does not exist.
func (db *DB) GetOrCreateSession(ctx context.Context, chatID int64) (int64, error) {
	var sessionID int64
	err := db.pool.QueryRow(ctx, `
		INSERT INTO chat_sessions (chat_id)
		VALUES ($1)
		ON CONFLICT (chat_id) DO UPDATE SET updated_at = NOW()
		RETURNING id
	`, chatID).Scan(&sessionID)
	if err != nil {
		return 0, fmt.Errorf("get or create session for chat %d: %w", chatID, err)
	}
	return sessionID, nil
}

// SaveMessage persists a message to the database.
func (db *DB) SaveMessage(ctx context.Context, sessionID int64, role, content string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO messages (session_id, role, content)
		VALUES ($1, $2, $3)
	`, sessionID, role, content)
	if err != nil {
		return fmt.Errorf("saving message: %w", err)
	}
	return nil
}

// GetRecentMessages returns the last n messages for the given session,
// ordered oldest first (suitable for passing directly to an LLM).
func (db *DB) GetRecentMessages(ctx context.Context, sessionID int64, n int) ([]Message, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, session_id, role, content, created_at
		FROM messages
		WHERE session_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, sessionID, n)
	if err != nil {
		return nil, fmt.Errorf("querying recent messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating messages: %w", err)
	}

	// Reverse to chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}
