package memory_plugin

import (
	"fmt"
	"log/slog"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/workflow/workflowspec"
	"github.com/vaastav/agentic_blueprint/ai_runtime/plugins/memory"
)

// MemoryStoreClient is the IR node for an InMemoryStore instance.
type MemoryStoreClient struct {
	golang.Service
	ir.IRNode
	service.ServiceNode

	Spec       *workflowspec.Service
	ClientName string
}

func newMemoryStoreClient(name string) (*MemoryStoreClient, error) {
	spec, err := workflowspec.GetService[memory.InMemoryStore]()
	if err != nil {
		return nil, err
	}
	return &MemoryStoreClient{Spec: spec, ClientName: name}, nil
}

// Implements ir.IRNode
func (node *MemoryStoreClient) Name() string {
	return node.ClientName
}

// Implements ir.IRNode
func (node *MemoryStoreClient) String() string {
	return node.Name() + " = InMemoryStore()"
}

// Implements golang.Instantiable
func (node *MemoryStoreClient) AddInstantiation(builder golang.NamespaceBuilder) error {
	if builder.Visited(node.ClientName) {
		return nil
	}

	slog.Info(fmt.Sprintf("Instantiating MemoryStoreClient %v in %v/%v", node.ClientName, builder.Info().Package.PackageName, builder.Info().FileName))

	constructor := node.Spec.Constructor.AsConstructor()
	return builder.DeclareConstructor(node.ClientName, constructor, []ir.IRNode{})
}

// Implements golang.ProvidesModule
func (node *MemoryStoreClient) AddToWorkspace(builder golang.WorkspaceBuilder) error {
	return node.Spec.AddToWorkspace(builder)
}

// Implements golang.ProvidesInterface
func (node *MemoryStoreClient) AddInterfaces(builder golang.ModuleBuilder) error {
	return node.Spec.AddToModule(builder)
}

// Implements service.ServiceNode
func (node *MemoryStoreClient) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	return node.Spec.Iface.ServiceInterface(ctx), nil
}

// Implements golang.Node
func (node *MemoryStoreClient) ImplementsGolangNode() {}

// MemoryAgentClient is the IR node for a MemoryAgent decorator instance.
type MemoryAgentClient struct {
	golang.Service
	ir.IRNode
	service.ServiceNode

	Spec        *workflowspec.Service
	ClientName  string
	InnerAgent  ir.IRNode
	MemoryStore ir.IRNode
}

func newMemoryAgentClient(name string, innerAgent ir.IRNode, memoryStore ir.IRNode) (*MemoryAgentClient, error) {
	spec, err := workflowspec.GetService[memory.MemoryAgent]()
	if err != nil {
		return nil, err
	}
	return &MemoryAgentClient{
		Spec:        spec,
		ClientName:  name,
		InnerAgent:  innerAgent,
		MemoryStore: memoryStore,
	}, nil
}

// Implements ir.IRNode
func (node *MemoryAgentClient) Name() string {
	return node.ClientName
}

// Implements ir.IRNode
func (node *MemoryAgentClient) String() string {
	return node.Name() + " = MemoryAgent(" + node.InnerAgent.Name() + ", " + node.MemoryStore.Name() + ")"
}

// Implements golang.Instantiable
func (node *MemoryAgentClient) AddInstantiation(builder golang.NamespaceBuilder) error {
	if builder.Visited(node.ClientName) {
		return nil
	}

	slog.Info(fmt.Sprintf("Instantiating MemoryAgentClient %v in %v/%v", node.ClientName, builder.Info().Package.PackageName, builder.Info().FileName))

	constructor := node.Spec.Constructor.AsConstructor()
	return builder.DeclareConstructor(node.ClientName, constructor, []ir.IRNode{node.InnerAgent, node.MemoryStore})
}

// Implements golang.ProvidesModule
func (node *MemoryAgentClient) AddToWorkspace(builder golang.WorkspaceBuilder) error {
	return node.Spec.AddToWorkspace(builder)
}

// Implements golang.ProvidesInterface
func (node *MemoryAgentClient) AddInterfaces(builder golang.ModuleBuilder) error {
	return node.Spec.AddToModule(builder)
}

// Implements service.ServiceNode
func (node *MemoryAgentClient) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	return node.Spec.Iface.ServiceInterface(ctx), nil
}

// Implements golang.Node
func (node *MemoryAgentClient) ImplementsGolangNode() {}
