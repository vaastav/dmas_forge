module github.com/vaastav/agentic_blueprint/examples/rag_chat/workflow

go 1.22.1

replace github.com/vaastav/agentic_blueprint/ai_runtime => ../../../ai_runtime

require (
	github.com/openai/openai-go v1.12.0
	github.com/vaastav/agentic_blueprint/ai_runtime v0.0.0-00010101000000-000000000000
)

require (
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
)
