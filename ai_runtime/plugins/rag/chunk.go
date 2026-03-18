package rag

import (
	"fmt"
	"maps"
	"strings"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

const (
	defaultChunkWordCount = 120
	defaultChunkOverlap   = 30
	defaultAutoQueryTopK  = 3
	defaultSearchTopK     = 5
)

type chunkRecord struct {
	id       string
	docID    string
	content  string
	metadata map[string]any
}

// chunkDocument uses a sliding window approach to split a document into chunks.
func chunkDocument(doc core.Document) ([]chunkRecord, error) {
	if strings.TrimSpace(doc.ID) == "" {
		return nil, fmt.Errorf("document ID cannot be empty")
	}

	content := strings.TrimSpace(doc.Content)
	if content == "" {
		return nil, fmt.Errorf("document %q has empty content", doc.ID)
	}

	words := strings.Fields(content)
	if len(words) == 0 {
		return nil, fmt.Errorf("document %q has empty content", doc.ID)
	}

	chunks := make([]chunkRecord, 0, max(1, len(words)/defaultChunkWordCount+1))
	step := defaultChunkWordCount - defaultChunkOverlap
	if step <= 0 {
		step = defaultChunkWordCount
	}

	for start := 0; start < len(words); start += step {
		end := start + defaultChunkWordCount
		if end > len(words) {
			end = len(words)
		}

		chunkText := strings.Join(words[start:end], " ")
		chunkID := fmt.Sprintf("%s#chunk-%03d", doc.ID, len(chunks))
		metadata := maps.Clone(doc.Metadata)
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["chunk_index"] = len(chunks)

		chunks = append(chunks, chunkRecord{
			id:       chunkID,
			docID:    doc.ID,
			content:  chunkText,
			metadata: metadata,
		})

		if end == len(words) {
			break
		}
	}

	return chunks, nil
}

func formatChunks(chunks []core.Chunk) string {
	if len(chunks) == 0 {
		return "No relevant knowledge found."
	}

	parts := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		parts = append(parts, fmt.Sprintf("[%d] source=%s score=%.4f\n%s", i+1, chunk.SourceDocID, chunk.Score, chunk.Content))
	}
	return strings.Join(parts, "\n\n")
}
