module github.com/vaastav/agentic_blueprint/examples/weather/wiring

go 1.22.1

require github.com/vaastav/agentic_blueprint/examples/weather/workflow v0.0.0

require github.com/vaastav/agentic_blueprint/ai_plugins v0.0.0

require (
	github.com/blueprint-uservices/blueprint/blueprint v0.0.0-20250729202253-a8f505263256
	github.com/blueprint-uservices/blueprint/plugins v0.0.0-20250729202253-a8f505263256
	github.com/vaastav/agentic_blueprint/ai_runtime v0.0.0 // indirect
)

require (
	github.com/blueprint-uservices/blueprint/runtime v0.0.0-20240405152959-f078915d2306 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/openai/openai-go v1.11.1 // indirect
	github.com/otiai10/copy v1.14.0 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	go.mongodb.org/mongo-driver v1.15.0 // indirect
	go.opentelemetry.io/otel v1.26.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.26.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.26.0 // indirect
	go.opentelemetry.io/otel/metric v1.26.0 // indirect
	go.opentelemetry.io/otel/sdk v1.26.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.26.0 // indirect
	go.opentelemetry.io/otel/trace v1.26.0 // indirect
	golang.org/x/exp v0.0.0-20240416160154-fe59bbe5cc7f // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/tools v0.20.0 // indirect
)

replace github.com/vaastav/agentic_blueprint/examples/weather/workflow => ../workflow

replace github.com/vaastav/agentic_blueprint/ai_runtime => ../../../ai_runtime
replace github.com/vaastav/agentic_blueprint/ai_plugins => ../../../ai_plugins
