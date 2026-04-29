package specs

import (
	"flag"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/jaeger"
	"github.com/blueprint-uservices/blueprint/plugins/opentelemetry"
	"github.com/blueprint-uservices/blueprint/plugins/workflow"

	"github.com/vaastav/agentic_blueprint/ai_plugins/model"
	"github.com/vaastav/agentic_blueprint/ai_plugins/openai_plugin"
	wf "github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow"
)

var mcpServers = flag.String("mcp-servers", "http://localhost:8080", "Comma-separated list of MCP server URLs for search/fetch tools")

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

	mcpServerURLs := *mcpServers

	collectorCore := openai_plugin.OpenAILLMAgent(spec, "collector_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	evaluatorCore := openai_plugin.OpenAILLMAgent(spec, "evaluator_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	analystCore := openai_plugin.OpenAILLMAgent(spec, "analyst_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	writerCore := openai_plugin.OpenAILLMAgent(spec, "writer_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	coordinatorCore := openai_plugin.OpenAILLMAgent(spec, "coordinator_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})
	researcherCore := openai_plugin.OpenAILLMAgent(spec, "researcher_core", minfo.URL, minfo.Key, minfo.Name, openai_plugin.AgentConfig{})

	services := financialServices{}
	services.collectorService = workflow.Service[wf.DataCollectorAgent](spec, "collector_service", collectorCore, mcpServerURLs)
	services.evaluatorService = workflow.Service[wf.DataEvaluatorAgent](spec, "evaluator_service", evaluatorCore)
	services.researchService = workflow.Service[wf.ResearchQualityController](spec, "research_quality_service", researcherCore, services.collectorService, services.evaluatorService)
	services.analystService = workflow.Service[wf.FinancialAnalystAgent](spec, "analyst_service", analystCore)
	services.writerService = workflow.Service[wf.ReportWriterAgent](spec, "writer_service", writerCore)
	services.coordinatorService = workflow.Service[wf.FinancialAnalyzerCoordinator](
		spec,
		"coordinator_service",
		coordinatorCore,
		services.researchService,
		services.analystService,
		services.writerService,
	)

	return services, nil
}

func (s financialServices) all() []string {
	return []string{s.collectorService, s.evaluatorService, s.researchService, s.analystService, s.writerService, s.coordinatorService}
}

func instrumentFinancialServices(spec wiring.WiringSpec, services financialServices) {
	collector := jaeger.Collector(spec, "jaeger")
	for _, serviceName := range services.all() {
		opentelemetry.Instrument(spec, serviceName, collector)
	}
}
