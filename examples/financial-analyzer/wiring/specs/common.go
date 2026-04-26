package specs

import (
	"flag"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/workflow"

	"github.com/vaastav/agentic_blueprint/ai_plugins/model"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow"
)

var mcpServers = flag.String("mcp-servers", "http://localhost:8080", "Comma-separated list of MCP server URLs for search/fetch tools")

const (
	defaultCompany = "Apple"
	defaultMode    = "sanity"
)

type financialServices struct {
	collectorService   string
	evaluatorService   string
	researchService    string
	analystService     string
	writerService      string
	coordinatorService string
}

func defineFinancialServices(spec wiring.WiringSpec) (financialServices, error) {
	minfo, err := model.GetModelInfo()
	if err != nil {
		return financialServices{}, err
	}

	company := defaultCompany
	mode := wf.NormalizeMode(defaultMode)
	mcpServerURLs := *mcpServers

	collectorCore := openai_plugin.OpenAILLMAgent(spec, "collector_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	evaluatorCore := openai_plugin.OpenAILLMAgent(spec, "evaluator_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	analystCore := openai_plugin.OpenAILLMAgent(spec, "analyst_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	writerCore := openai_plugin.OpenAILLMAgent(spec, "writer_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	coordinatorCore := openai_plugin.OpenAILLMAgent(spec, "coordinator_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	researcherCore := openai_plugin.OpenAILLMAgent(spec, "researcher_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})

	services := financialServices{}
	services.collectorService = workflow.Service[wf.DataCollectorAgent](spec, "collector_service", collectorCore, mcpServerURLs, company, mode)
	services.evaluatorService = workflow.Service[wf.DataEvaluatorAgent](spec, "evaluator_service", evaluatorCore, company, mode)
	services.researchService = workflow.Service[wf.ResearchQualityController](spec, "research_quality_service", researcherCore, services.collectorService, services.evaluatorService, company, mode)
	services.analystService = workflow.Service[wf.FinancialAnalystAgent](spec, "analyst_service", analystCore, company, mode)
	services.writerService = workflow.Service[wf.ReportWriterAgent](spec, "writer_service", writerCore, company, mode)
	services.coordinatorService = workflow.Service[wf.FinancialAnalyzerCoordinator](
		spec,
		"coordinator_service",
		coordinatorCore,
		services.researchService,
		services.analystService,
		services.writerService,
		company,
		mode,
	)

	return services, nil
}
