package vllm

import (
	"fmt"
	"log/slog"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/address"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/workflow/workflowspec"
	"github.com/vaastav/agentic_blueprint/ai_runtime/plugins/openaiagent"
)

type VLLMClient struct {
	golang.Service
	ir.IRNode
	service.ServiceNode

	ClientName string
	ModelName  string
	APIKey     string
	DialAddr   *address.DialConfig
	Spec       *workflowspec.Service
}

func newVLLMClient(name string, addr *address.DialConfig, model string, apikey string) (*VLLMClient, error) {
	spec, err := workflowspec.GetService[openaiagent.OpenAILLMClient]()
	if err != nil {
		return nil, err
	}
	return &VLLMClient{Spec: spec, DialAddr: addr, ClientName: name, ModelName: model, APIKey: apikey}, nil
}

// Implements ir.IRNode
func (node *VLLMClient) Name() string {
	return node.ClientName
}

// Implements ir.IRNode
func (node *VLLMClient) String() string {
	return node.Name() + " = VLLMClient(" + node.DialAddr.Name() + ")"
}

// Implements golang.ProvidesModule
func (node *VLLMClient) AddToWorkspace(builder golang.WorkspaceBuilder) error {
	return node.Spec.AddToWorkspace(builder)
}

// Implements golang.ProvidesInterface
func (node *VLLMClient) AddInterfaces(builder golang.ModuleBuilder) error {
	return node.Spec.AddToModule(builder)
}

// Implements service.ServiceNode
func (node *VLLMClient) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	return node.Spec.Iface.ServiceInterface(ctx), nil
}

// Implements golang.Instantiable
func (node *VLLMClient) AddInstantiation(builder golang.NamespaceBuilder) error {
	if builder.Visited(node.ClientName) {
		return nil
	}

	slog.Info(fmt.Sprintf("Instantiating VLLMClient %v in %v/%v", node.ClientName, builder.Info().Package.PackageName, builder.Info().FileName))

	constructor := node.Spec.Constructor.AsConstructor()
	return builder.DeclareConstructor(node.ClientName, constructor, []ir.IRNode{node.DialAddr, &ir.IRValue{Value: node.APIKey}, &ir.IRValue{Value: node.ModelName}})
}

// Implements golang.Node
func (node *VLLMClient) ImplementsGolangNode()    {}
func (node *VLLMClient) ImplementsGolangService() {}
