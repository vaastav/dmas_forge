package mcp

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
	mcpClientName := serviceName + ".mcp_client"
	mcpServerName := serviceName + ".mcp_server"
	mcpAddrName := serviceName + ".mcp.addr"

	ptr := pointer.GetPointer(spec, serviceName)
	if ptr == nil {
		slog.Error("Unable to deploy " + serviceName + " using MCP as it is not a pointer")
		return
	}

	address.Define[*mcpServer](spec, mcpAddrName, mcpServerName)

	clientNext := ptr.AddSrcModifier(spec, mcpClientName)
	spec.Define(mcpClientName, &mcpClient{}, func(namespace wiring.Namespace) (ir.IRNode, error) {
		addr, err := address.Dial[*mcpServer](namespace, clientNext)
		if err != nil {
			return nil, blueprint.Errorf("GRPC client %s expected %s to be an address, but encountered %s", mcpClientName, clientNext, err)
		}
		return newMcpClient(mcpClientName, addr)
	})

	// Add the server-side modifier, which is an address that PointsTo the mcpServerName
	serverNext := ptr.AddAddrModifier(spec, mcpAddrName)
	spec.Define(mcpServerName, &mcpServer{}, func(namespace wiring.Namespace) (ir.IRNode, error) {
		var wrapped golang.Service
		if err := namespace.Get(serverNext, &wrapped); err != nil {
			return nil, blueprint.Errorf("GRPC server %s expected %s to be a golang.Service, but encountered %s", mcpServerName, serverNext, err)
		}

		server, err := newMcpServer(mcpServerName, wrapped)
		if err != nil {
			return nil, err
		}

		err = address.Bind[*mcpServer](namespace, mcpAddrName, server, &server.Bind)
		server.Bind.PreferredPort = 12345
		return server, err
	})
}
