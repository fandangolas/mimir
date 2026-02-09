package chunker

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChunker_ChunkText_ShortText(t *testing.T) {
	c := NewChunker(512, 50)
	text := "This is a short message."

	chunks := c.ChunkText(text)

	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for short text, got %d", len(chunks))
	}

	if chunks[0] != text {
		t.Errorf("expected chunk to equal original text")
	}
}

func TestChunker_ChunkText_LongText(t *testing.T) {
	c := NewChunker(50, 5) // Very small limits for testing (50 tokens = 150 chars)

	// Create text that will definitely need chunking
	// Each sentence is ~60 chars, so 3 sentences = 180 chars > 150 char limit
	sentences := []string{
		"This is the first sentence with enough words to be long.",
		"This is the second sentence with enough words to be long.",
		"This is the third sentence with enough words to be long.",
		"This is the fourth sentence with enough words to be long.",
	}
	text := strings.Join(sentences, " ")

	chunks := c.ChunkText(text)

	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for long text, got %d", len(chunks))
	}

	// Verify each chunk is within size limit (with some tolerance for separators)
	maxChars := 50 * 3 // maxTokens * 3 (conservative estimate)
	for i, chunk := range chunks {
		charCount := utf8.RuneCountInString(chunk)
		if charCount > maxChars+30 { // +30 for separator tolerance
			t.Errorf("chunk %d exceeds max size: %d chars (max %d)", i, charCount, maxChars)
		}
	}

	// Verify chunks cover all content (by joining and checking key words)
	combined := strings.Join(chunks, "")
	if !strings.Contains(combined, "first") || !strings.Contains(combined, "third") {
		t.Errorf("chunks don't cover all original content")
	}
}

func TestChunker_ChunkText_ParagraphSplitting(t *testing.T) {
	c := NewChunker(30, 5) // Very small limit: 30 tokens = 90 chars

	// Text with paragraph breaks (should split on \n\n first)
	// Each repetition is ~35 chars, so 4 reps = 140 chars > 90 char limit
	text := strings.Repeat("Paragraph one with some content.\n\n", 4) +
		strings.Repeat("Paragraph two with some content.\n\n", 4)

	chunks := c.ChunkText(text)

	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}

	// Chunks should preferably break on paragraph boundaries
	for i, chunk := range chunks {
		if len(chunk) == 0 {
			t.Errorf("chunk %d is empty", i)
		}
	}
}

func TestChunker_DefaultValues(t *testing.T) {
	tests := []struct {
		name             string
		maxTokens        int
		overlapTokens    int
		expectedMax      int
		expectedOverlap  int
	}{
		{
			name:            "zero max tokens uses default",
			maxTokens:       0,
			overlapTokens:   50,
			expectedMax:     512,
			expectedOverlap: 50,
		},
		{
			name:            "negative overlap uses zero",
			maxTokens:       100,
			overlapTokens:   -10,
			expectedMax:     100,
			expectedOverlap: 0,
		},
		{
			name:            "overlap >= max uses 10% of max",
			maxTokens:       100,
			overlapTokens:   150,
			expectedMax:     100,
			expectedOverlap: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChunker(tt.maxTokens, tt.overlapTokens)

			if c.maxTokens != tt.expectedMax {
				t.Errorf("expected maxTokens=%d, got %d", tt.expectedMax, c.maxTokens)
			}

			if c.overlapTokens != tt.expectedOverlap {
				t.Errorf("expected overlapTokens=%d, got %d", tt.expectedOverlap, c.overlapTokens)
			}
		})
	}
}

func TestChunker_Overlap(t *testing.T) {
	c := NewChunker(50, 10) // 50 tokens max, 10 token overlap

	// Create text that requires multiple chunks
	text := strings.Repeat("word ", 200) // 200 words ~= 800 chars ~= 200 tokens

	chunks := c.ChunkText(text)

	if len(chunks) < 3 {
		t.Errorf("expected at least 3 chunks for test to be meaningful, got %d", len(chunks))
	}

	// Check that consecutive chunks have overlap
	for i := 0; i < len(chunks)-1; i++ {
		chunk1 := chunks[i]
		chunk2 := chunks[i+1]

		// Get end of first chunk (approximate overlap region)
		overlapChars := 10 * 3 // overlapTokens * 3 (conservative estimate)
		if len(chunk1) < overlapChars {
			continue
		}

		endOfFirst := chunk1[len(chunk1)-overlapChars:]

		// Check if chunk2 starts with similar content
		// (exact match may not happen due to separator handling, so check for common words)
		commonWords := 0
		words1 := strings.Fields(endOfFirst)
		words2 := strings.Fields(chunk2)

		for _, w1 := range words1 {
			for _, w2 := range words2 {
				if w1 == w2 {
					commonWords++
					break
				}
			}
		}

		if commonWords == 0 {
			t.Errorf("chunks %d and %d appear to have no overlap", i, i+1)
		}
	}
}
