package workflow

import (
	"context"
	"fmt"
)

type campaignRunState struct {
	Request CampaignRequest
	Result  *CampaignResult
}

type campaignRunStateKey struct{}

func withCampaignRunState(ctx context.Context, state *campaignRunState) context.Context {
	return context.WithValue(ctx, campaignRunStateKey{}, state)
}

func campaignRunStateFromContext(ctx context.Context) (*campaignRunState, error) {
	state, ok := ctx.Value(campaignRunStateKey{}).(*campaignRunState)
	if !ok || state == nil {
		return nil, fmt.Errorf("marketing campaign request state missing from context")
	}
	return state, nil
}
