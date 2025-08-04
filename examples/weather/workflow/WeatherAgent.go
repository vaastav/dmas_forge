package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	openai "github.com/openai/openai-go"
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
	get_weather_tool := openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "get_weather",
			Description: openai.String("Get weather at the given location"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]string{
						"type": "string",
					},
				},
				"required": []string{"location"},
			},
		},
	}
	tools := make(map[string]openai.ChatCompletionToolParam)
	tools["get_weather"] = get_weather_tool
	err = a.agent.AddTools(ctx, tools)
	if err != nil {
		return nil, err
	}
	// Register tool function
	err = a.agent.RegisterToolCallHandler(ctx, toolHandler)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func toolHandler(ctx context.Context, toolcall openai.ChatCompletionMessageToolCall) (string, error) {
	log.Println("Inside tool call")
	if toolcall.Function.Name == "get_weather" {
		log.Println("Tool Call into get_weather")
		var args map[string]interface{}
		err := json.Unmarshal([]byte(toolcall.Function.Arguments), &args)
		if err != nil {
			return "", err
		}
		location := args["location"].(string)
		return "Weather in " + location + " is 30 degrees Celsius!", nil
	}
	return "Incorrect tool call oops!", errors.New("Unsupported tool call")
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
