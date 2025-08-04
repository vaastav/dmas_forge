package openai_plugin

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/pointer"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
)

func OpenAILLMAgent(spec wiring.WiringSpec, agent_name string, url string, apikey string, model_name string) string {

	agentBackendName := agent_name + ".openai_agent"

	spec.Define(agentBackendName, &AgentClient{}, func(namespace wiring.Namespace) (ir.IRNode, error) {
		return newAgentClient(agent_name, url, apikey, model_name)
	})

	pointer.CreatePointer[*AgentClient](spec, agent_name, agentBackendName)

	return agent_name
}
