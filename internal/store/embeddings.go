package store

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
)

// SaveEmbedding stores an embedding for a message.
// If an embedding already exists for this message, it will be updated.
func (db *DB) SaveEmbedding(ctx context.Context, messageID int64, embedding []float32) error {
	vector := pgvector.NewVector(embedding)

	_, err := db.pool.Exec(ctx, `
		INSERT INTO message_embeddings (message_id, embedding)
		VALUES ($1, $2)
		ON CONFLICT (message_id) DO UPDATE
		SET embedding = $2, created_at = NOW()
	`, messageID, vector)

	if err != nil {
		return fmt.Errorf("saving embedding for message %d: %w", messageID, err)
	}

	return nil
}

// GetMessagesWithoutEmbeddings returns messages that don't have embeddings yet.
// Useful for backfilling embeddings for existing messages.
func (db *DB) GetMessagesWithoutEmbeddings(ctx context.Context, limit int) ([]Message, error) {
	query := `
		SELECT m.id, m.session_id, m.role, m.content, m.created_at
		FROM messages m
		LEFT JOIN message_embeddings me ON me.message_id = m.id
		WHERE me.id IS NULL
		ORDER BY m.id
		LIMIT $1
	`

	rows, err := db.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying messages without embeddings: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}
		messages = append(messages, m)
	}

	return messages, rows.Err()
}

// CountMessagesWithoutEmbeddings returns the number of messages without embeddings.
func (db *DB) CountMessagesWithoutEmbeddings(ctx context.Context) (int64, error) {
	var count int64
	err := db.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM messages m
		LEFT JOIN message_embeddings me ON me.message_id = m.id
		WHERE me.id IS NULL
	`).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("counting messages without embeddings: %w", err)
	}

	return count, nil
}
