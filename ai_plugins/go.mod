module github.com/vaastav/agentic_blueprint/ai_plugins

go 1.22.1

require (
	github.com/blueprint-uservices/blueprint/blueprint v0.0.0-20250729202253-a8f505263256
	github.com/blueprint-uservices/blueprint/plugins v0.0.0-20250729202253-a8f505263256
)

require github.com/vaastav/agentic_blueprint/ai_runtime v0.0.0

replace github.com/vaastav/agentic_blueprint/ai_runtime => ../ai_runtime

require (
	github.com/openai/openai-go v1.11.1 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	golang.org/x/exp v0.0.0-20240416160154-fe59bbe5cc7f // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/tools v0.20.0 // indirect
)
