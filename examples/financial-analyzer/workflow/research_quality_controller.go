package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/vaastav/dmas_forge/ai_runtime/core"
)

type ResearchQualityControllerImpl struct {
	agent     core.Agent
	collector DataCollectorAgent
	evaluator DataEvaluatorAgent
}

func NewResearchQualityControllerImpl(
	ctx context.Context,
	agent core.Agent,
	collector DataCollectorAgent,
	evaluator DataEvaluatorAgent,
) (ResearchQualityController, error) {
	_ = ctx
	return &ResearchQualityControllerImpl{
		agent:     agent,
		collector: collector,
		evaluator: evaluator,
	}, nil
}

func (c *ResearchQualityControllerImpl) RefineResearch(ctx context.Context, req ResearchRequest) (ResearchQualityResult, error) {
	company, mode, err := requireCompanyAndMode(req.Company, req.Mode)
	if err != nil {
		return ResearchQualityResult{}, err
	}
	minRating, maxRefinements := controllerThresholds(mode)

	aggregate := ResearchQualityResult{}

	current, err := c.collector.CollectData(ctx, CollectionRequest{
		Company: company,
		Mode:    mode,
		Query:   initialCollectionQuery(company, mode),
	})
	if err != nil {
		return ResearchQualityResult{}, fmt.Errorf("initial collection failed: %w", err)
	}
	aggregate.ResearchMarkdown = current.ResearchMarkdown

	for attempt := 0; ; attempt++ {
		select {
		case <-ctx.Done():
			return aggregate, ctx.Err()
		default:
		}

		evaluation, err := c.evaluator.EvaluateData(ctx, EvaluationRequest{
			Company:          company,
			Mode:             mode,
			ResearchMarkdown: current.ResearchMarkdown,
		})
		if err != nil {
			return ResearchQualityResult{}, fmt.Errorf("evaluation failed: %w", err)
		}
		aggregate.FinalRating = evaluation.OverallRating

		if MeetsMinRating(evaluation.OverallRating, minRating) || attempt >= maxRefinements {
			break
		}

		aggregate.RefinementCount++
		current, err = c.collector.CollectData(ctx, CollectionRequest{
			Company:       company,
			Mode:          mode,
			Query:         refinementCollectionQuery(company, mode, evaluation),
			PriorResearch: aggregate.ResearchMarkdown,
		})
		if err != nil {
			return aggregate, fmt.Errorf("refinement collection failed: %w", err)
		}
		aggregate.ResearchMarkdown = current.ResearchMarkdown
	}

	if strings.TrimSpace(aggregate.ResearchMarkdown) == "" {
		return aggregate, fmt.Errorf("research output was empty")
	}

	return aggregate, nil
}

func controllerThresholds(mode string) (QualityRating, int) {
	if NormalizeMode(mode) == ModeFull {
		return RatingGood, 3
	}
	return RatingFair, 1
}

func initialCollectionQuery(company, mode string) string {
	if NormalizeMode(mode) == ModeFull {
		return fmt.Sprintf(`Create a high-quality research brief for %s.

Gather current market performance, the latest quarterly earnings results, recent news and developments, and key valuation metrics.

Ask for:
- current stock price and recent movement
- latest quarterly earnings results and performance vs expectations
- recent news and developments
- key metrics such as P/E ratio and market cap

Prefer current, reputable financial sources and include exact figures and citations where available. Return a complete markdown research package.`, company)
	}

	return fmt.Sprintf(`Create a quick sanity-check research snapshot for %s.

Gather:
- today's stock price, change percentage, and trading volume
- latest EPS and revenue actual vs estimate
- 2 timely headlines with URLs
- key valuation metrics such as P/E ratio and market cap

The goal is to produce trustworthy data with minimal latency. Keep the research compact, factual, and source-backed. Return markdown only.`, company)
}

func refinementCollectionQuery(company, mode string, evaluation EvaluationRecord) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Improve the research brief for %s.\n", company))
	builder.WriteString(fmt.Sprintf("Run mode: %s\n", mode))
	minRating, _ := controllerThresholds(mode)
	builder.WriteString(fmt.Sprintf("The previous research was rated %s. Minimum required rating: %s.\n\n", evaluation.OverallRating, minRating))
	builder.WriteString("Address the evaluator feedback below and return a stronger revised research package.\n\n")
	builder.WriteString("Evaluator feedback:\n")
	builder.WriteString(strings.TrimSpace(evaluation.Feedback))
	builder.WriteString("\n\nReturn a revised markdown research package with better evidence, fresher sourcing, and more complete coverage where possible.")
	return builder.String()
}
