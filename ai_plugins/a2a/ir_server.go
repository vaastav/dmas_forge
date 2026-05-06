package a2a

import (
	"fmt"
	"reflect"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/blueprint"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/address"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
	"github.com/blueprint-uservices/blueprint/plugins/golang/gocode"
	"github.com/vaastav/dmas_forge/ai_plugins/a2a/a2acodegen"
)

// IRNode representing a Golang A2A server.
// This node does not introduce any new runtime interfaces or types that can be used by other IRNodes.
type golangA2AServer struct {
	service.ServiceNode
	golang.GeneratesFuncs
	golang.Instantiable

	InstanceName string
	Bind         *address.BindConfig
	Wrapped      golang.Service

	outputPackage string
}

// Represents a service that is exposed over HTTP
type A2AInterface struct {
	service.ServiceInterface
	Wrapped service.ServiceInterface
}

func (i *A2AInterface) GetName() string {
	return "a2a(" + i.Wrapped.GetName() + ")"
}

func (i *A2AInterface) GetMethods() []service.Method {
	return i.Wrapped.GetMethods()
}

func newA2AServer(name string, wrapped ir.IRNode) (*golangA2AServer, error) {
	service, is_service := wrapped.(golang.Service)
	if !is_service {
		return nil, blueprint.Errorf("A2A server %s expected %s to be a golang service, but got %s", name, wrapped.Name(), reflect.TypeOf(wrapped).String())
	}

	node := &golangA2AServer{}
	node.InstanceName = name
	node.Wrapped = service
	node.outputPackage = "a2a"
	return node, nil
}

func (n *golangA2AServer) String() string {
	return n.InstanceName + " = A2AServer(" + n.Wrapped.Name() + ", " + n.Bind.Name() + ")"
}

func (n *golangA2AServer) Name() string {
	return n.InstanceName
}

// Generates the HTTP Server handler
func (node *golangA2AServer) GenerateFuncs(builder golang.ModuleBuilder) error {
	iface, err := golang.GetGoInterface(builder, node.Wrapped)
	if err != nil {
		return err
	}

	err = a2acodegen.GenerateServerHandler(builder, iface, node.outputPackage)
	if err != nil {
		return err
	}
	return nil
}

func (node *golangA2AServer) AddInstantiation(builder golang.NamespaceBuilder) error {
	// Only generate instantiation code for this instance once
	if builder.Visited(node.InstanceName) {
		return nil
	}

	iface, err := golang.GetGoInterface(builder, node.Wrapped)
	if err != nil {
		return err
	}

	constructor := &gocode.Constructor{
		Package: builder.Module().Info().Name + "/" + node.outputPackage,
		Func: gocode.Func{
			Name: fmt.Sprintf("New_%v_A2AServerHandler", iface.BaseName),
			Arguments: []gocode.Variable{
				{Name: "ctx", Type: &gocode.UserType{Package: "context", Name: "Context"}},
				{Name: "service", Type: iface},
				{Name: "serverAddr", Type: &gocode.BasicType{Name: "string"}},
			},
		},
	}
	return builder.DeclareConstructor(node.InstanceName, constructor, []ir.IRNode{node.Wrapped, node.Bind})
}

func (node *golangA2AServer) GetInterface(ctx ir.BuildContext) (service.ServiceInterface, error) {
	iface, err := node.Wrapped.GetInterface(ctx)
	return &A2AInterface{Wrapped: iface}, err
}

func (node *golangA2AServer) ImplementsGolangNode() {}
