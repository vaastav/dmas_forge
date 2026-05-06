package openai_plugin

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/pointer"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
)

// AgentConfig holds optional configuration for an OpenAI LLM agent.
// Zero values indicate use of defaults.
type AgentConfig struct {
	// MaxToolRounds is the maximum number of tool-call round-trips allowed
	// in a single LLMCallWithTools invocation.
	// If set to 0 (or unset), the default value of 10 is used.
	MaxToolRounds int

	// FailOnToolHandlerError preserves fail-fast behavior for tool handler errors.
	// If false, handler errors are returned to the model as tool messages until
	// the final allowed tool round.
	FailOnToolHandlerError bool
}

// OpenAILLMAgent creates an OpenAI LLM agent with the given configuration.
// The config parameter allows customization of agent behavior; zero values
// in config fields will use sensible defaults.
func OpenAILLMAgent(spec wiring.WiringSpec, agent_name string, url string, apikey string, model_name string, config AgentConfig) string {

	agentBackendName := agent_name + ".openai_agent"

	spec.Define(agentBackendName, &AgentClient{}, func(namespace wiring.Namespace) (ir.IRNode, error) {
		return newAgentClient(agent_name, url, apikey, model_name, config.MaxToolRounds, config.FailOnToolHandlerError)
	})

	pointer.CreatePointer[*AgentClient](spec, agent_name, agentBackendName)

	return agent_name
}
