package workflow

import (
	"context"
	"fmt"
	"strings"
)

type financialRunState struct {
	Company string
	Mode    string
	Result  *AnalysisResult
}

type financialRunStateKey struct{}

func withFinancialRunState(ctx context.Context, state *financialRunState) context.Context {
	return context.WithValue(ctx, financialRunStateKey{}, state)
}

func financialRunStateFromContext(ctx context.Context) (*financialRunState, error) {
	state, ok := ctx.Value(financialRunStateKey{}).(*financialRunState)
	if !ok || state == nil {
		return nil, fmt.Errorf("financial analyzer request state missing from context")
	}
	return state, nil
}

func requireCompanyAndMode(company, mode string) (string, string, error) {
	company = strings.TrimSpace(company)
	mode = strings.TrimSpace(mode)

	validationErrors := []string{}

	if company == "" {
		validationErrors = append(validationErrors, "\"company\" must be provided and cannot be empty.")
	}

	if mode == "" || (mode != ModeSanity && mode != ModeFull) {
		validationErrors = append(validationErrors, "\"mode\" must be provided as \"sanity\" or \"full\".")
	}

	if len(validationErrors) > 0 {
		return "", "", fmt.Errorf("invalid request: %s", strings.Join(validationErrors, " "))
	}

	return company, mode, nil
}
