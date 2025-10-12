package a2a

import (
	"log/slog"

	"github.com/blueprint-uservices/blueprint/blueprint/pkg/blueprint"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/address"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/coreplugins/pointer"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/ir"
	"github.com/blueprint-uservices/blueprint/blueprint/pkg/wiring"
	"github.com/blueprint-uservices/blueprint/plugins/golang"
)

func Deploy(spec wiring.WiringSpec, serviceName string) {
	// The nodes that we are defining
	a2aClient := serviceName + ".a2a_client"
	a2aServer := serviceName + ".a2a_server"
	a2aAddr := serviceName + ".a2a.addr"

	// Get the pointer metadata
	ptr := pointer.GetPointer(spec, serviceName)
	if ptr == nil {
		slog.Error("Unable to deploy " + serviceName + " using HTTP as it not a pointer")
	}

	// Define the address that will be used by clients and the server
	address.Define[*golangA2AServer](spec, a2aAddr, a2aServer)

	// Add the client-side modifier
	//
	// The client-side modifier creates an HTTP client and dials the server address.
	// It assumes that the next src modifier node will be a golangHttpServer address.
	clientNext := ptr.AddSrcModifier(spec, a2aClient)
	spec.Define(a2aClient, &golangA2AClient{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		addr, err := address.Dial[*golangA2AServer](ns, clientNext)
		if err != nil {
			return nil, blueprint.Errorf("HTTP client %s expected %s to be an address, but encountered %s", a2aClient, clientNext, err)
		}
		return newA2AClient(a2aClient, addr)
	})

	// Add the server-side modifier, which is an address that PointsTo the grpcServer
	serverNext := ptr.AddAddrModifier(spec, a2aAddr)
	spec.Define(a2aServer, &golangA2AServer{}, func(ns wiring.Namespace) (ir.IRNode, error) {
		var wrapped golang.Service
		if err := ns.Get(serverNext, &wrapped); err != nil {
			return nil, blueprint.Errorf("HTTP server %s expected %s to be a golang.Service, but encountered %s", a2aServer, serverNext, err)
		}

		server, err := newA2AServer(a2aServer, wrapped)
		if err != nil {
			return nil, err
		}

		err = address.Bind[*golangA2AServer](ns, a2aAddr, server, &server.Bind)
		return server, err
	})
}
