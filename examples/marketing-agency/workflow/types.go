package workflow

import "context"

type CampaignRequest struct {
	BrandName      string   `json:"brand_name"`
	Keywords       []string `json:"keywords"`
	Description    string   `json:"description"`
	TargetAudience string   `json:"target_audience"`
}

type CampaignResult struct {
	Domains           []string          `json:"domains"`
	SelectedDomain    string            `json:"selected_domain"`
	WebsiteFiles      map[string]string `json:"website_files"`
	MarketingStrategy string            `json:"marketing_strategy"`
	LogoFilepath      string            `json:"logo_filepath"`
	Summary           string            `json:"summary"`
}

type BrandInfo struct {
	Name           string
	Description    string
	Keywords       []string
	TargetAudience string
}

type WebsiteContent struct {
	Files map[string]string `json:"files"`
}

type DomainAgent interface {
	SuggestDomains(ctx context.Context, keywords []string) ([]string, error)
}

type WebsiteAgent interface {
	GenerateWebsite(ctx context.Context, domain string, brandInfo BrandInfo) (WebsiteContent, error)
}

type MarketingAgent interface {
	CreateStrategy(ctx context.Context, domain string, brandInfo BrandInfo) (string, error)
}

type LogoAgent interface {
	GenerateLogo(ctx context.Context, brandName, style string) (string, error)
}

type MarketingCoordinator interface {
	CreateCampaign(ctx context.Context, req CampaignRequest) (CampaignResult, error)
}
