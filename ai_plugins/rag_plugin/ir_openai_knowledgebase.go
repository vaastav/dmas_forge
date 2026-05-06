package rag_plugin

import (
	"fmt"
	"log/slog"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/workflow/workflowspec"
	ragruntime "github.com/vaastav/dmas_forge/ai_runtime/plugins/rag"
)

// OpenAIKnowledgeBaseClient is the Blueprint IR node for an OpenAI-backed
// knowledge base. It uses OpenAI's embedding API and delegates vector storage
// to a pluggable VectorStore implementation.
type OpenAIKnowledgeBaseClient struct {
	golang.Service
	ir.IRNode
	service.ServiceNode

	Spec           *workflowspec.Service
	ClientName     string
	BaseURL        string
	APIKey         string
	EmbeddingModel string
	VectorStore    ir.IRNode
}

func newOpenAIKnowledgeBaseClient(name string, baseURL string, apiKey string, embeddingModel string, vectorStore ir.IRNode) (*OpenAIKnowledgeBaseClient, error) {
	spec, err := workflowspec.GetService[ragruntime.OpenAIKnowledgeBase]()
	if err != nil {
		return nil, err
	}
	return &OpenAIKnowledgeBaseClient{
		Spec:           spec,
		ClientName:     name,
		BaseURL:        baseURL,
		APIKey:         apiKey,
		EmbeddingModel: embeddingModel,
		VectorStore:    vectorStore,
	}, nil
}

func (node *OpenAIKnowledgeBaseClient) Name() string {
	return node.ClientName
}

func (node *OpenAIKnowledgeBaseClient) String() string {
	return node.Name() + " = OpenAIKnowledgeBase(" + node.VectorStore.Name() + ")"
}

func (node *OpenAIKnowledgeBaseClient) AddInstantiation(builder golang.NamespaceBuilder) error {
	if builder.Visited(node.ClientName) {
		return nil
	}

	slog.Info(fmt.Sprintf("Instantiating OpenAIKnowledgeBaseClient %v in %v/%v", node.ClientName, builder.Info().Package.PackageName, builder.Info().FileName))

	constructor := node.Spec.Constructor.AsConstructor()
	return builder.DeclareConstructor(node.ClientName, constructor, []ir.IRNode{
		&ir.IRValue{Value: node.BaseURL},
		&ir.IRValue{Value: node.APIKey},
		&ir.IRValue{Value: node.EmbeddingModel},
		node.VectorStore,
	})
}

func (node *OpenAIKnowledgeBaseClient) AddToWorkspace(builder golang.WorkspaceBuilder) error {
	return node.Spec.AddToWorkspace(builder)
}

func (node *OpenAIKnowledgeBaseClient) AddInterfaces(builder golang.ModuleBuilder) error {
	return node.Spec.AddToModule(builder)
}

func (node *OpenAIKnowledgeBaseClient) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	return node.Spec.Iface.ServiceInterface(ctx), nil
}

func (node *OpenAIKnowledgeBaseClient) ImplementsGolangNode() {}
