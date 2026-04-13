package workflow

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"github.com/vaastav/agentic_blueprint/examples/marketing-agency/workflow/tools"
)

type LogoAgentImpl struct {
	agent core.Agent
}

func NewLogoAgentImpl(ctx context.Context, agent core.Agent, outputDir, apiKey, baseURL string) (LogoAgent, error) {
	a := &LogoAgentImpl{agent: agent}
	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL))

	if err := a.agent.AddSystemPrompt(ctx, LogoAgentPrompt); err != nil {
		return nil, err
	}

	if err := a.agent.AddTools(ctx, map[string]openai.ChatCompletionToolParam{
		"generate_image": tools.ImageGenTool(),
	}); err != nil {
		return nil, err
	}

	if err := a.agent.RegisterToolCallHandler(ctx, tools.ImageGenHandler(&client, outputDir)); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *LogoAgentImpl) GenerateLogo(ctx context.Context, brandName, style string) (string, error) {
	query := fmt.Sprintf(
		"Brand: %s\\nStyle: %s\\nUse generate_image and return JSON filepath.",
		brandName,
		style,
	)

	resp, err := a.agent.LLMCallWithTools(ctx, query)
	if err != nil {
		return "", err
	}

	var payload struct {
		Filepath string `json:"filepath"`
	}
	if unmarshalJSONFromLLMResponse(resp, &payload) && strings.TrimSpace(payload.Filepath) != "" {
		return payload.Filepath, nil
	}

	return extractFilepathFallback(resp), nil
}

func extractFilepathFallback(raw string) string {
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		v := strings.TrimSpace(line)
		if strings.Contains(v, "artifacts/") && strings.HasSuffix(v, ".png") {
			return v
		}
	}
	return ""
}
