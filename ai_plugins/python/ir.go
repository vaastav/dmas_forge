package python

import (
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/service"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
)

type (
	// Node should be implemented by any IRNode that ought to exist within a Python namespace
	Node interface {
		ir.IRNode
		ImplementsPythonNode() // Idiomatically necessary in Go for typecasting correctly
	}

	// Service is a [Node] that represents a callable service with an interface, constructor, and methods.
	// For example, services within a workflow spec are represented by Service nodes because they have invokable methods.
	// This is the python counterpart to Blueprint's golang.Service
	//
	// Service nodes must implement the [Instantiable] and [ProvidesInterface] interfaces.
	Service interface {
		Node
		Instantiable
		ProvidesInterface
		service.ServiceNode
		ImplementsPythonService() // Idiomatically necessary in Go for typecasting
	}
)

type (
	// A [Node] should implement Instantiable if it wants to instantiate objects in the generated python namespace at runtime.
	// For example, a service node needs to actually call the service constructor at runtime, to instantiate the service
	Instantiable interface {
		// AddInstantiation is invoked during compilation to allow the callee to provide code snippets for instantiating a golang object, using the provided [NamespaceBuilder]
		AddInstantiation(NamespaceBuilder) error
	}

	// A [Node] should implement ProvidesInterface if it wants to modify or extend any service interfaces,
	// particularly those that are defined by other nodes. For example, a tracing plugin might extend all methods
	// of an interface to add trace contexts.
	ProvidesInterface interface {
		AddInterfaces(ModuleBuilder) error
	}
)

type NamespaceBuilder interface {
	ir.BuildContext
}

type ModuleBuilder interface {
	ir.BuildContext
}
