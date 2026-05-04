package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	openai "github.com/openai/openai-go"
	"github.com/vaastav/agentic_blueprint/ai_runtime/core"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const domainCheckTracerName = "github.com/vaastav/agentic_blueprint/examples/marketing-agency/workflow/tools/domain_check"

func DomainCheckTool() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        "check_domain",
			Description: openai.String("Check whether a domain appears available by connecting to port 443 or 80."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"domain": map[string]interface{}{
						"type":        "string",
						"description": "Domain name to check, without protocol",
					},
				},
				"required": []string{"domain"},
			},
		},
	}
}

func DomainCheckHandler() core.ToolHandlerFn {
	return func(ctx context.Context, tc openai.ChatCompletionMessageToolCall) (string, error) {
		mode := "real"
		if benchmarkMockEnabled() {
			mode = "mock"
		}
		tracer := trace.SpanFromContext(ctx).TracerProvider().Tracer(domainCheckTracerName)
		ctx, span := tracer.Start(ctx, "tool.domain_check",
			trace.WithAttributes(
				attribute.String("tool.name", "check_domain"),
				attribute.String("provider_mode", mode),
			),
		)
		defer span.End()

		if tc.Function.Name != "check_domain" {
			err := fmt.Errorf("unsupported tool: %s", tc.Function.Name)
			recordToolError(span, err)
			return "", err
		}

		var args struct {
			Domain string `json:"domain"`
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			recordToolError(span, err)
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		domain := strings.TrimSpace(args.Domain)
		if domain == "" {
			err := fmt.Errorf("domain cannot be empty")
			recordToolError(span, err)
			return "", err
		}

		available := domainAvailable(ctx, domain)
		span.SetAttributes(
			attribute.String("domain.name", domain),
			attribute.Bool("domain.available", available),
		)
		span.SetStatus(codes.Ok, "")

		b, err := json.Marshal(map[string]interface{}{
			"domain":    domain,
			"available": available,
		})
		if err != nil {
			recordToolError(span, err)
			return "", fmt.Errorf("marshal domain check: %w", err)
		}

		return string(b), nil
	}
}

func domainAvailable(ctx context.Context, domain string) bool {
	if benchmarkMockEnabled() {
		return true
	}
	return !tcpConnects(ctx, domain, "443") && !tcpConnects(ctx, domain, "80")
}

func tcpConnects(ctx context.Context, domain, port string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(domain, port))
	if err != nil {
		return false
	}
	defer conn.Close()

	return true
}

func benchmarkMockEnabled() bool {
	return os.Getenv("DMAS_BENCH_MOCK") == "1"
}

func recordToolError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
