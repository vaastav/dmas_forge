package mcp

import (
	"fmt"
	"log/slog"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/blueprint"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/address"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/golang/gocode"
	"github.com/vaastav/dmas_forge/ai_plugins/mcp/mcpcodegen"
)

type mcpClient struct {
	golang.Service
	golang.GeneratesFuncs

	InstanceName string
	ServerAddr   *address.Address[*mcpServer]

	outputPackage string
}

func newMcpClient(name string, addr *address.Address[*mcpServer]) (*mcpClient, error) {
	node := &mcpClient{}
	node.InstanceName = name
	node.ServerAddr = addr
	node.outputPackage = "mcp"

	return node, nil
}

func (n *mcpClient) String() string {
	return n.InstanceName + " = MCPClient(" + n.ServerAddr.Dial.Name() + ")"
}

func (n *mcpClient) Name() string {
	return n.InstanceName
}

func (node *mcpClient) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	iface, err := node.ServerAddr.Server.GetInterface(ctx)
	if err != nil {
		return nil, err
	}
	mcp, isGrpc := iface.(*mcpInterface)
	if !isGrpc {
		return nil, blueprint.Errorf("mcp client expected a MCP interface from %v but found %v", node.ServerAddr.Server.Name(), iface)
	}
	wrapped, isValid := mcp.Wrapped.(*gocode.ServiceInterface)
	if !isValid {
		return nil, blueprint.Errorf("mcp client expected the server's MCP interface to wrap a gocode interface but found %v", mcp)
	}
	return wrapped, nil
}

// Just makes sure that the interface exposed by the server is included in the built module
func (node *mcpClient) AddInterfaces(builder golang.ModuleBuilder) error {
	return node.ServerAddr.Server.Wrapped.AddInterfaces(builder)
}

// Generates proto files and the RPC client
func (node *mcpClient) GenerateFuncs(builder golang.ModuleBuilder) error {
	// Get the service that we are wrapping
	iface, err := golang.GetGoInterface(builder, node)
	if err != nil {
		return err
	}

	// Only generate mcp client instantiation code for this service once
	if builder.Visited(iface.Name + ".mcp.client") {
		return nil
	}

	// Generate the RPC client
	err = mcpcodegen.GenerateClient(builder, iface, node.outputPackage)
	if err != nil {
		return err
	}

	return nil
}

func (node *mcpClient) AddInstantiation(builder golang.NamespaceBuilder) error {
	// Only generate instantiation code for this instance once
	if builder.Visited(node.InstanceName) {
		return nil
	}

	// Get the service that we are wrapping
	iface, err := golang.GetGoInterface(builder, node)
	if err != nil {
		return err
	}

	constructor := &gocode.Constructor{
		Package: builder.Module().Info().Name + "/" + node.outputPackage,
		Func: gocode.Func{
			Name: fmt.Sprintf("New_%v_MCPClient", iface.BaseName),
			Arguments: []gocode.Variable{
				{Name: "ctx", Type: &gocode.UserType{Package: "context", Name: "Context"}},
				{Name: "addr", Type: &gocode.BasicType{Name: "string"}},
			},
		},
	}

	slog.Info(fmt.Sprintf("Instantiating MCPClient %v in %v/%v", node.InstanceName, builder.Info().Package.PackageName, builder.Info().FileName))
	return builder.DeclareConstructor(node.InstanceName, constructor, []ir.IRNode{node.ServerAddr.Dial})
}

func (node *mcpClient) ImplementsGolangNode()    {}
func (node *mcpClient) ImplementsGolangService() {}
