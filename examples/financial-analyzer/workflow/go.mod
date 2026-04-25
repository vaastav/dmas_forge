module github.com/vaastav/agentic_blueprint/examples/financial-analyzer/workflow

go 1.23.0

require (
	github.com/modelcontextprotocol/go-sdk v1.3.0
	github.com/openai/openai-go v1.11.1
	github.com/vaastav/agentic_blueprint/ai_runtime v0.0.0
)

require (
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
)

replace github.com/vaastav/agentic_blueprint/ai_runtime => ../../../ai_runtime
