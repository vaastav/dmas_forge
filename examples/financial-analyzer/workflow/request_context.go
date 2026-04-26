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
	if company == "" {
		return "", "", fmt.Errorf("company is required")
	}

	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "", "", fmt.Errorf("mode is required")
	}
	if mode != ModeSanity && mode != ModeFull {
		return "", "", fmt.Errorf("mode must be either %q or %q", ModeSanity, ModeFull)
	}

	return company, mode, nil
}
