package chunker

import (
	"strings"
	"unicode/utf8"
)

// Chunker splits long text into smaller chunks with overlap.
type Chunker struct {
	maxTokens     int
	overlapTokens int
}

// NewChunker creates a new chunker with the specified token limits.
// maxTokens is the maximum size of each chunk.
// overlapTokens is the number of tokens to overlap between chunks.
func NewChunker(maxTokens, overlapTokens int) *Chunker {
	if maxTokens <= 0 {
		maxTokens = 512
	}
	if overlapTokens < 0 {
		overlapTokens = 0
	}
	if overlapTokens >= maxTokens {
		overlapTokens = maxTokens / 10 // Default to 10% overlap
	}

	return &Chunker{
		maxTokens:     maxTokens,
		overlapTokens: overlapTokens,
	}
}

// ChunkText splits text into chunks if it exceeds maxTokens.
// Returns a slice of chunks (may be just one if text is short).
func (c *Chunker) ChunkText(text string) []string {
	// Conservative approximation: 1 token ≈ 3 characters for English
	// (using 3 instead of 4 to ensure we don't exceed token limits)
	maxChars := c.maxTokens * 3
	overlapChars := c.overlapTokens * 3

	if utf8.RuneCountInString(text) <= maxChars {
		return []string{text}
	}

	// Split by natural boundaries (in priority order)
	separators := []string{"\n\n", "\n", ". ", "! ", "? ", "; ", ", ", " "}

	return c.recursiveSplit(text, separators, maxChars, overlapChars)
}

func (c *Chunker) recursiveSplit(text string, separators []string, maxChars, overlapChars int) []string {
	if len(separators) == 0 {
		// Fallback: split by character count
		return c.splitByChars(text, maxChars, overlapChars)
	}

	sep := separators[0]
	parts := strings.Split(text, sep)

	var chunks []string
	var current strings.Builder

	for _, part := range parts {
		// Try adding this part to current chunk
		candidate := current.String()
		if current.Len() > 0 {
			candidate += sep
		}
		candidate += part

		if utf8.RuneCountInString(candidate) > maxChars {
			// Current chunk is full, save it
			if current.Len() > 0 {
				chunks = append(chunks, current.String())

				// Start new chunk with overlap from previous
				overlapText := c.getOverlap(current.String(), overlapChars)
				current.Reset()
				current.WriteString(overlapText)
			}

			// If single part is too large, recurse with next separator
			if utf8.RuneCountInString(part) > maxChars {
				subChunks := c.recursiveSplit(part, separators[1:], maxChars, overlapChars)
				chunks = append(chunks, subChunks...)
				current.Reset()
			} else {
				if current.Len() > 0 {
					current.WriteString(sep)
				}
				current.WriteString(part)
			}
		} else {
			current.Reset()
			current.WriteString(candidate)
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

func (c *Chunker) getOverlap(text string, overlapChars int) string {
	runes := []rune(text)
	if len(runes) <= overlapChars {
		return text
	}
	return string(runes[len(runes)-overlapChars:])
}

func (c *Chunker) splitByChars(text string, maxChars, overlapChars int) []string {
	runes := []rune(text)
	var chunks []string

	for i := 0; i < len(runes); i += maxChars - overlapChars {
		end := i + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))

		if end >= len(runes) {
			break
		}
	}

	return chunks
}
