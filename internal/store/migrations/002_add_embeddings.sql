-- Enable pgvector extension for vector similarity search
CREATE EXTENSION IF NOT EXISTS vector;

-- Table to store message embeddings
-- One-to-one relationship with messages table
CREATE TABLE IF NOT EXISTS message_embeddings (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL UNIQUE REFERENCES messages(id) ON DELETE CASCADE,
    embedding VECTOR(768) NOT NULL,  -- nomic-embed-text dimensions
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add full-text search support to messages table
-- Generated column automatically updates when content changes
ALTER TABLE messages
ADD COLUMN IF NOT EXISTS content_tsv TSVECTOR
GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;

-- Create HNSW index for semantic similarity search
-- This provides fast approximate nearest neighbor search
-- Parameters:
--   m = 16: Number of links per node (higher = more accurate but larger index)
--   ef_construction = 64: Quality during index build (higher = better quality, slower build)
CREATE INDEX IF NOT EXISTS message_embeddings_hnsw_idx ON message_embeddings
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- Create GIN index for full-text keyword search
CREATE INDEX IF NOT EXISTS messages_fts_idx ON messages USING GIN(content_tsv);

-- Create index on message_id for efficient joins
CREATE INDEX IF NOT EXISTS idx_message_embeddings_message_id ON message_embeddings(message_id);

-- Create composite index for filtering by conversation and recency
CREATE INDEX IF NOT EXISTS idx_messages_conversation_created ON messages(session_id, created_at DESC);
