package mcpcodegen

import "github.com/blueprint-uservices/blueprint/plugins/golang"

// RequireMCPSDK ensures the modelcontextprotocol Go SDK is added as a module
// dependency once.
func RequireMCPSDK(builder golang.ModuleBuilder) {
	if !builder.Visited("mcp.sdk.dependency") {
		builder.Require("github.com/modelcontextprotocol/go-sdk", "v1.3.0")
	}
}
