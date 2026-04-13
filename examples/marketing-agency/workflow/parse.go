package workflow

import (
	"encoding/json"
	"regexp"
	"strings"
)

// unmarshalJSONFromLLMResponse attempts to extract JSON from an LLM response.
// It tries the full response first, then any ```json code blocks.
func unmarshalJSONFromLLMResponse(raw string, out interface{}) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}

	// Try the full response directly.
	if json.Unmarshal([]byte(trimmed), out) == nil {
		return true
	}

	// Try each ```json code block.
	for _, block := range extractJSONCodeBlocks(trimmed) {
		if json.Unmarshal([]byte(block), out) == nil {
			return true
		}
	}

	return false
}

var codeBlockRE = regexp.MustCompile("(?is)```(?:json|javascript|js)?\\s*(.*?)\\s*```")

func extractJSONCodeBlocks(raw string) []string {
	matches := codeBlockRE.FindAllStringSubmatch(raw, -1)
	blocks := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			blocks = append(blocks, m[1])
		}
	}
	return blocks
}
