package mcp

import (
	"fmt"
	"log/slog"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/address"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/golang/gocode"
	"github.com/vaastav/agentic_blueprint/ai_plugins/mcp/mcpcodegen"
)

type mcpServer struct {
	service.ServiceNode
	golang.GeneratesFuncs
	golang.Instantiable

	InstanceName string
	Bind         *address.BindConfig
	Wrapped      golang.Service

	outputPackage string
}

// Represents a service that is exposed over mcp
type mcpInterface struct {
	service.ServiceInterface
	Wrapped service.ServiceInterface
}

func (mcp *mcpInterface) GetName() string {
	return "mcp(" + mcp.Wrapped.GetName() + ")"
}

func (mcp *mcpInterface) GetMethods() []service.Method {
	return mcp.Wrapped.GetMethods()
}

func newMcpServer(name string, service golang.Service) (*mcpServer, error) {
	node := &mcpServer{}
	node.InstanceName = name
	node.Wrapped = service
	node.outputPackage = "mcp"
	return node, nil
}

func (n *mcpServer) String() string {
	return n.InstanceName + " = MCPServer(" + n.Wrapped.Name() + ", " + n.Bind.Name() + ")"
}

func (n *mcpServer) Name() string {
	return n.InstanceName
}

// Generates proto files and the RPC server handler
func (node *mcpServer) GenerateFuncs(builder golang.ModuleBuilder) error {
	// Get the service that we are wrapping
	iface, err := golang.GetGoInterface(builder, node.Wrapped)
	if err != nil {
		return err
	}

	// Only generate mcp server instantiation code for this service once
	if builder.Visited(iface.Name + ".mcp.server") {
		return nil
	}

	// Generate the MCP server handler
	err = mcpcodegen.GenerateServerHandler(builder, iface, node.outputPackage)
	if err != nil {
		return err
	}

	return nil
}

func (node *mcpServer) AddInstantiation(builder golang.NamespaceBuilder) error {
	// Only generate instantiation code for this instance once
	if builder.Visited(node.InstanceName) {
		return nil
	}

	iface, err := golang.GetGoInterface(builder.Module(), node.Wrapped)
	if err != nil {
		return err
	}

	constructor := &gocode.Constructor{
		Package: builder.Module().Info().Name + "/" + node.outputPackage,
		Func: gocode.Func{
			Name: fmt.Sprintf("New_%v_MCPServerHandler", iface.BaseName),
			Arguments: []gocode.Variable{
				{Name: "ctx", Type: &gocode.UserType{Package: "context", Name: "Context"}},
				{Name: "service", Type: iface},
				{Name: "serverAddr", Type: &gocode.BasicType{Name: "string"}},
			},
		},
	}

	slog.Info(fmt.Sprintf("Instantiating mcpServer %v in %v/%v", node.InstanceName, builder.Info().Package.PackageName, builder.Info().FileName))
	return builder.DeclareConstructor(node.InstanceName, constructor, []ir.IRNode{node.Wrapped, node.Bind})
}

func (node *mcpServer) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	iface, err := node.Wrapped.GetInterface(ctx)
	return &mcpInterface{Wrapped: iface}, err
}
func (node *mcpServer) ImplementsGolangNode() {}
