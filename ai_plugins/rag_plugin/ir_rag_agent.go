package rag_plugin

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/workflow/workflowspec"
	ragruntime "github.com/vaastav/agentic_blueprint/ai_runtime/plugins/rag"
)

type RAGAgentClient struct {
	golang.Service
	ir.IRNode
	service.ServiceNode

	Spec          *workflowspec.Service
	ClientName    string
	InnerAgent    ir.IRNode
	KnowledgeBase ir.IRNode
	ToolExposure  ToolExposure
	AutoQuery     bool
	TopK          int
	AutoIndex     bool
}

func newRAGAgentClient(name string, innerAgent ir.IRNode, knowledgeBase ir.IRNode, toolExposure ToolExposure, autoQuery bool, topK int, autoIndex bool) (*RAGAgentClient, error) {
	spec, err := workflowspec.GetService[ragruntime.RAGAgent]()
	if err != nil {
		return nil, err
	}
	return &RAGAgentClient{
		Spec:          spec,
		ClientName:    name,
		InnerAgent:    innerAgent,
		KnowledgeBase: knowledgeBase,
		ToolExposure:  toolExposure,
		AutoQuery:     autoQuery,
		TopK:          topK,
		AutoIndex:     autoIndex,
	}, nil
}

func (node *RAGAgentClient) Name() string {
	return node.ClientName
}

func (node *RAGAgentClient) String() string {
	return node.Name() + " = RAGAgent(" + node.InnerAgent.Name() + ", " + node.KnowledgeBase.Name() + ")"
}

func (node *RAGAgentClient) AddInstantiation(builder golang.NamespaceBuilder) error {
	if builder.Visited(node.ClientName) {
		return nil
	}

	slog.Info(fmt.Sprintf("Instantiating RAGAgentClient %v in %v/%v", node.ClientName, builder.Info().Package.PackageName, builder.Info().FileName))

	constructor := node.Spec.Constructor.AsConstructor()
	return builder.DeclareConstructor(node.ClientName, constructor, []ir.IRNode{
		node.InnerAgent,
		node.KnowledgeBase,
		&ir.IRValue{Value: strconv.Itoa(int(node.ToolExposure))},
		&ir.IRValue{Value: strconv.FormatBool(node.AutoQuery)},
		&ir.IRValue{Value: strconv.Itoa(node.TopK)},
		&ir.IRValue{Value: strconv.FormatBool(node.AutoIndex)},
	})
}

func (node *RAGAgentClient) AddToWorkspace(builder golang.WorkspaceBuilder) error {
	return node.Spec.AddToWorkspace(builder)
}

func (node *RAGAgentClient) AddInterfaces(builder golang.ModuleBuilder) error {
	return node.Spec.AddToModule(builder)
}

func (node *RAGAgentClient) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	return node.Spec.Iface.ServiceInterface(ctx), nil
}

func (node *RAGAgentClient) ImplementsGolangNode() {}
