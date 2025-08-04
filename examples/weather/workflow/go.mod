module github.com/vaastav/agentic_blueprint/examples/weather/workflow

go 1.22.1

require github.com/vaastav/agentic_blueprint/ai_runtime v0.0.0

require (
	github.com/openai/openai-go v1.11.1 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
)

replace github.com/vaastav/agentic_blueprint/ai_runtime => ../../../ai_runtime
