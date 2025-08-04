package workflow

import (
	"context"

	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

type WeatherAgent interface {
	Query(ctx context.Context, query string) (string, error)
}

type WeatherAgentImpl struct {
	agent    core.Agent
	disAgent DisasterAgent
}

func NewWeatherAgentImpl(ctx context.Context, agent core.Agent, disasterAgent DisasterAgent) (WeatherAgent, error) {

	a := &WeatherAgentImpl{agent: agent, disAgent: disasterAgent}
	system_prompt := "Act as a weather analyst and prediction service. GIven a user query about weather in a given location, generate a weather report. Feel free to use the provided tools if necessary. "

	err := a.agent.AddSystemPrompt(ctx, system_prompt)
	if err != nil {
		return nil, err
	}

	// TODO: Add tools

	return a, nil
}

func (a *WeatherAgentImpl) Query(ctx context.Context, query string) (string, error) {
	result, err := a.agent.LLMCallWithTools(ctx, query)
	if err != nil {
		return "", err
	}
	disaster_result, err := a.disAgent.Query(ctx, result)
	if err != nil {
		return result, err
	}
	return result + "\n" + disaster_result, nil
}
