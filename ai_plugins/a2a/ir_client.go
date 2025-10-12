package a2a

import (
	"fmt"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/blueprint"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/address"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/golang/gocode"
	"github.com/vaastav/agentic_blueprint/ai_plugins/a2a/a2acodegen"
)

type golangA2AClient struct {
	golang.Node
	golang.Service
	golang.GeneratesFuncs
	golang.Instantiable

	InstanceName string
	ServerAddr   *address.Address[*golangA2AServer]

	outputPackage string
}

func newA2AClient(name string, addr *address.Address[*golangA2AServer]) (*golangA2AClient, error) {
	node := &golangA2AClient{}
	node.InstanceName = name
	node.ServerAddr = addr
	node.outputPackage = "a2a"

	return node, nil
}

func (n *golangA2AClient) String() string {
	return n.InstanceName + " = A2AClient(" + n.ServerAddr.Dial.Name() + ")"
}

func (n *golangA2AClient) Name() string {
	return n.InstanceName
}

func (node *golangA2AClient) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	iface, err := node.ServerAddr.Server.GetInterface(ctx)
	if err != nil {
		return nil, err
	}
	a2a, isA2A := iface.(*A2AInterface)
	if !isA2A {
		return nil, blueprint.Errorf("a2a client expected an HTTP interface from %v but found %v", node.ServerAddr.Name(), iface)
	}
	wrapped, isValid := a2a.Wrapped.(*gocode.ServiceInterface)
	if !isValid {
		return nil, blueprint.Errorf("a2a client expected the server's HTTP interface to wrap a gocode interface but found %v", a2a)
	}
	return wrapped, nil
}

// Just makes sure that the interface exposed by the server is included in the built module
func (node *golangA2AClient) AddInterfaces(builder golang.ModuleBuilder) error {
	return node.ServerAddr.Server.Wrapped.AddInterfaces(builder)
}

func (node *golangA2AClient) GenerateFuncs(builder golang.ModuleBuilder) error {
	if builder.Visited(node.InstanceName + ".generateFuncs") {
		return nil
	}

	iface, err := golang.GetGoInterface(builder, node)
	if err != nil {
		return err
	}

	return a2acodegen.GenerateClient(builder, iface, node.outputPackage)
}

func (node *golangA2AClient) AddInstantiation(builder golang.NamespaceBuilder) error {
	// Only generate instantiation code for this instance once
	if builder.Visited(node.InstanceName) {
		return nil
	}

	iface, err := golang.GetGoInterface(builder, node)
	if err != nil {
		return err
	}

	constructor := &gocode.Constructor{
		Package: builder.Module().Info().Name + "/" + node.outputPackage,
		Func: gocode.Func{
			Name: fmt.Sprintf("New_%v_A2AClient", iface.BaseName),
			Arguments: []gocode.Variable{
				{Name: "ctx", Type: &gocode.UserType{Package: "context", Name: "Context"}},
				{Name: "addr", Type: &gocode.BasicType{Name: "string"}},
			},
		},
	}

	return builder.DeclareConstructor(node.InstanceName, constructor, []ir.IRNode{node.ServerAddr.Dial})
}

func (node *golangA2AClient) ImplementsGolangNode()    {}
func (node *golangA2AClient) ImplementsGolangService() {}
