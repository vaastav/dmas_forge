module github.com/vaastav/dmas_forge/examples/financial-analyzer/wiring

go 1.23.0

require github.com/vaastav/dmas_forge/examples/financial-analyzer/workflow v0.0.0

require (
	github.com/blueprint-uservices/blueprint/runtime v0.0.0-20240405152959-f078915d2306 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/modelcontextprotocol/go-sdk v1.3.0 // indirect
	github.com/openai/openai-go v1.11.1 // indirect
	github.com/otiai10/copy v1.14.0 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/vaastav/dmas_forge/ai_runtime v0.0.0-20260506010313-a64618e2c60b // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go.mongodb.org/mongo-driver v1.15.0 // indirect
	go.opentelemetry.io/otel v1.26.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.26.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.26.0 // indirect
	go.opentelemetry.io/otel/metric v1.26.0 // indirect
	go.opentelemetry.io/otel/sdk v1.26.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.26.0 // indirect
	go.opentelemetry.io/otel/trace v1.26.0 // indirect
	golang.org/x/exp v0.0.0-20240416160154-fe59bbe5cc7f // indirect
	golang.org/x/mod v0.25.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.15.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/tools v0.34.0 // indirect
)

require (
	github.com/blueprint-uservices/blueprint/blueprint v0.0.0-20250729202253-a8f505263256
	github.com/blueprint-uservices/blueprint/plugins v0.0.0-20250729202253-a8f505263256
	github.com/vaastav/dmas_forge/ai_plugins v0.0.0-20260506011127-3725bf4e6864
)

replace github.com/vaastav/dmas_forge/examples/financial-analyzer/workflow => ../workflow
