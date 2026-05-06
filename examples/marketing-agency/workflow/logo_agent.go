package workflow

import (
	"context"
	"fmt"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/vaastav/dmas_forge/ai_runtime/core"
	"github.com/vaastav/dmas_forge/examples/marketing-agency/workflow/tools"
)

const logoAgentPrompt = `You are a brand designer.

Task:
- Create a strong logo generation prompt from brand context.
- Use generate_image tool exactly once.
- After the tool succeeds, return only the JSON metadata from the tool.
`

type LogoAgentImpl struct {
	agent core.Agent
}

func NewLogoAgentImpl(ctx context.Context, agent core.Agent, apiKey, baseURL string) (LogoAgent, error) {
	a := &LogoAgentImpl{agent: agent}
	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL))

	if err := a.agent.AddSystemPrompt(ctx, logoAgentPrompt); err != nil {
		return nil, err
	}

	if err := a.agent.AddTools(ctx, map[string]openai.ChatCompletionToolParam{
		"generate_image": tools.ImageGenTool(),
	}); err != nil {
		return nil, err
	}

	if err := a.agent.RegisterToolCallHandler(ctx, tools.ImageGenHandler(&client)); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *LogoAgentImpl) GenerateLogo(ctx context.Context, brandName, style string) (string, error) {
	query := fmt.Sprintf(
		"Brand: %s\\nStyle: %s\\nUse generate_image to create a logo. Return only the tool's JSON metadata.",
		brandName,
		style,
	)

	output, err := a.agent.LLMCallWithTools(ctx, query)
	if err != nil {
		return "", err
	}

	return output, nil
}
