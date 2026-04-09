package workflow

import "strings"

func extractJSONPayload(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return raw
	}
	return raw[start : end+1]
}
