package workflow

import (
	"encoding/json"
	"regexp"
	"strings"
)

func unmarshalJSONFromLLMResponse(raw string, out interface{}) bool {
	for _, candidate := range jsonCandidates(raw) {
		if tryUnmarshalJSON(candidate, out) {
			return true
		}
	}
	return false
}

func jsonCandidates(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	add(trimmed)
	for _, block := range extractJSONCodeBlocks(trimmed) {
		add(block)
	}

	return out
}

func tryUnmarshalJSON(candidate string, out interface{}) bool {
	if err := json.Unmarshal([]byte(candidate), out); err == nil {
		return true
	}

	for i := 0; i < len(candidate); i++ {
		if candidate[i] != '{' && candidate[i] != '[' {
			continue
		}

		dec := json.NewDecoder(strings.NewReader(candidate[i:]))
		var payload json.RawMessage
		if err := dec.Decode(&payload); err != nil {
			continue
		}

		if err := json.Unmarshal(payload, out); err == nil {
			return true
		}
	}

	return false
}

var codeBlockRE = regexp.MustCompile("(?is)```(?:json|javascript|js)?\\s*(.*?)\\s*```")

func extractJSONCodeBlocks(raw string) []string {
	matches := codeBlockRE.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return nil
	}

	blocks := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			blocks = append(blocks, m[1])
		}
	}

	return blocks
}
