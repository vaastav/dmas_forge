package specs

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/workflow"

	"github.com/vaastav/agentic_blueprint/ai_plugins/model"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/marketing-agency/workflow"
)

type marketingServices struct {
	domainService      string
	websiteService     string
	marketingService   string
	logoService        string
	coordinatorService string
}

func defineMarketingServices(spec wiring.WiringSpec) (marketingServices, error) {
	model, err := model.GetModelInfo()
	if err != nil {
		return marketingServices{}, err
	}

	domainAgentCore := openai_plugin.OpenAILLMAgent(
		spec,
		"domain_agent_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)
	websiteAgentCore := openai_plugin.OpenAILLMAgent(
		spec,
		"website_agent_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)
	marketingAgentCore := openai_plugin.OpenAILLMAgent(
		spec,
		"marketing_agent_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)
	logoAgentCore := openai_plugin.OpenAILLMAgent(
		spec,
		"logo_agent_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)
	coordinatorCore := openai_plugin.OpenAILLMAgent(
		spec,
		"coordinator_core",
		model.URL,
		model.Key,
		model.Name,
		openai_plugin.AgentConfig{},
	)

	services := marketingServices{}
	services.domainService = workflow.Service[wf.DomainAgent](spec, "domain_service", domainAgentCore)
	services.websiteService = workflow.Service[wf.WebsiteAgent](spec, "website_service", websiteAgentCore)
	services.marketingService = workflow.Service[wf.MarketingAgent](spec, "marketing_service", marketingAgentCore)
	services.logoService = workflow.Service[wf.LogoAgent](spec, "logo_service", logoAgentCore, model.Key, model.URL)
	services.coordinatorService = workflow.Service[wf.MarketingCoordinator](
		spec,
		"coordinator_service",
		coordinatorCore,
		services.domainService,
		services.websiteService,
		services.marketingService,
		services.logoService,
	)

	return services, nil
}
