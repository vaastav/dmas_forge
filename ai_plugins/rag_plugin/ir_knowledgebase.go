package rag_plugin

import (
	"fmt"
	"log/slog"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/workflow/workflowspec"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
)

// KnowledgeBaseClient is the Blueprint IR node for a custom KnowledgeBase
// implementation. It is instantiated via KnowledgeBase[Impl] wiring function.
type KnowledgeBaseClient struct {
	golang.Service
	ir.IRNode
	service.ServiceNode

	Spec       *workflowspec.Service
	ClientName string
}

func newKnowledgeBaseClient[Impl core.KnowledgeBase](name string) (*KnowledgeBaseClient, error) {
	spec, err := workflowspec.GetService[Impl]()
	if err != nil {
		return nil, err
	}
	return &KnowledgeBaseClient{Spec: spec, ClientName: name}, nil
}

func (node *KnowledgeBaseClient) Name() string {
	return node.ClientName
}

func (node *KnowledgeBaseClient) String() string {
	return node.Name() + " = " + node.Spec.Constructor.Name + "()"
}

func (node *KnowledgeBaseClient) AddInstantiation(builder golang.NamespaceBuilder) error {
	if builder.Visited(node.ClientName) {
		return nil
	}

	slog.Info(fmt.Sprintf("Instantiating KnowledgeBaseClient %v in %v/%v", node.ClientName, builder.Info().Package.PackageName, builder.Info().FileName))

	constructor := node.Spec.Constructor.AsConstructor()
	return builder.DeclareConstructor(node.ClientName, constructor, []ir.IRNode{})
}

func (node *KnowledgeBaseClient) AddToWorkspace(builder golang.WorkspaceBuilder) error {
	return node.Spec.AddToWorkspace(builder)
}

func (node *KnowledgeBaseClient) AddInterfaces(builder golang.ModuleBuilder) error {
	return node.Spec.AddToModule(builder)
}

func (node *KnowledgeBaseClient) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	return node.Spec.Iface.ServiceInterface(ctx), nil
}

func (node *KnowledgeBaseClient) ImplementsGolangNode() {}
