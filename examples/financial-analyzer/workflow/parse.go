package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var codeBlockRE = regexp.MustCompile("(?is)```(?:json|javascript|js)?\\s*(.*?)\\s*```")
var overallRatingRE = regexp.MustCompile(`(?im)(?:overall\s*rating|overall)\s*:\s*(EXCELLENT|GOOD|FAIR|POOR)\b`)

func unmarshalJSONFromLLMResponse(raw string, out interface{}) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}

	if json.Unmarshal([]byte(trimmed), out) == nil {
		return true
	}

	for _, block := range extractJSONCodeBlocks(trimmed) {
		if json.Unmarshal([]byte(block), out) == nil {
			return true
		}
	}

	return false
}

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

func marshalJSON(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseEvaluationResponse(raw string) (EvaluationRecord, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return EvaluationRecord{}, fmt.Errorf("evaluator response was empty")
	}

	var parsed EvaluationRecord
	if unmarshalJSONFromLLMResponse(trimmed, &parsed) {
		parsed.Completeness = NormalizeQualityRating(string(parsed.Completeness))
		parsed.Accuracy = NormalizeQualityRating(string(parsed.Accuracy))
		parsed.Currency = NormalizeQualityRating(string(parsed.Currency))
		parsed.OverallRating = NormalizeQualityRating(string(parsed.OverallRating))
		parsed.Feedback = strings.TrimSpace(parsed.Feedback)
		parsed.Raw = trimmed
		if parsed.OverallRating == "" {
			parsed.OverallRating = deriveOverallRating(parsed.Completeness, parsed.Accuracy, parsed.Currency)
		}
		if parsed.Feedback == "" {
			parsed.Feedback = trimmed
		}
		return parsed, nil
	}

	parsed = EvaluationRecord{
		OverallRating: extractOverallRating(trimmed),
		Feedback:      strings.TrimSpace(trimmed),
		Raw:           trimmed,
	}

	if parsed.OverallRating == RatingPoor {
		parsed.OverallRating = deriveOverallRating(parsed.Completeness, parsed.Accuracy, parsed.Currency)
	}
	return parsed, nil
}

func extractOverallRating(raw string) QualityRating {
	matches := overallRatingRE.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return NormalizeQualityRating(matches[1])
	}
	return RatingPoor
}

func deriveOverallRating(ratings ...QualityRating) QualityRating {
	overall := RatingExcellent
	for _, rating := range ratings {
		normalized := NormalizeQualityRating(string(rating))
		if normalized == RatingPoor {
			return RatingPoor
		}
		if normalized == RatingFair {
			overall = RatingFair
			continue
		}
		if normalized == RatingGood && overall == RatingExcellent {
			overall = RatingGood
		}
	}
	return overall
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
