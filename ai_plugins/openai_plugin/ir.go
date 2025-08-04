package openai_plugin

import (
	"fmt"
	"log/slog"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/workflow/workflowspec"
	"github.com/vaastav/agentic_blueprint/ai_runtime/plugins/openaiagent"
)

type AgentClient struct {
	golang.Service
	ir.IRNode
	service.ServiceNode

	Spec       *workflowspec.Service
	ClientName string
	URL        string
	Key        string
	Model      string
}

func newAgentClient(name string, url string, key string, model string) (*AgentClient, error) {
	spec, err := workflowspec.GetService[openaiagent.OpenAILLMClient]()
	if err != nil {
		return nil, err
	}
	return &AgentClient{Spec: spec, ClientName: name, URL: url, Key: key, Model: model}, nil
}

// Implements ir.IRNode
func (node *AgentClient) Name() string {
	return node.ClientName
}

// Implements ir.IRNode
func (node *AgentClient) String() string {
	return node.Name() + " = OpenAILLMClient()"
}

// Implements golang.Instantiable
func (node *AgentClient) AddInstantiation(builder golang.NamespaceBuilder) error {
	if builder.Visited(node.ClientName) {
		return nil
	}

	slog.Info(fmt.Sprintf("Instantiating AgentClient %v in %v/%v", node.ClientName, builder.Info().Package.PackageName, builder.Info().FileName))

	constructor := node.Spec.Constructor.AsConstructor()
	return builder.DeclareConstructor(node.ClientName, constructor, []ir.IRNode{&ir.IRValue{Value: node.URL}, &ir.IRValue{Value: node.Key}, &ir.IRValue{Value: node.Model}})
}

// Implements golang.ProvidesModule
func (node *AgentClient) AddToWorkspace(builder golang.WorkspaceBuilder) error {
	return node.Spec.AddToWorkspace(builder)
}

// Implements golang.ProvidesInterface
func (node *AgentClient) AddInterfaces(builder golang.ModuleBuilder) error {
	return node.Spec.AddToModule(builder)
}

// Implements service.ServiceNode
func (node *AgentClient) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	return node.Spec.Iface.ServiceInterface(ctx), nil
}

// Implements golang.Node
func (node *AgentClient) ImplementsGolangNode() {}
