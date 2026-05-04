#!/usr/bin/env bash
set -euo pipefail

root="${1:-.}"
otel_version=v1.26.0

find "$root" -name go.mod -type f -print0 |
while IFS= read -r -d '' mod_file; do
	grep -q '^module blueprint/goproc/' "$mod_file" || continue

	printf 'pinning otel dependencies in %s\n' "$mod_file"
	(
		cd "$(dirname "$mod_file")"
		go mod edit \
			-require=go.opentelemetry.io/otel@"$otel_version" \
			-require=go.opentelemetry.io/otel/metric@"$otel_version" \
			-require=go.opentelemetry.io/otel/trace@"$otel_version" \
			-require=go.opentelemetry.io/otel/sdk@"$otel_version" \
			-require=go.opentelemetry.io/otel/sdk/metric@"$otel_version" \
			-require=go.opentelemetry.io/otel/exporters/stdout/stdoutmetric@"$otel_version" \
			-require=go.opentelemetry.io/otel/exporters/stdout/stdouttrace@"$otel_version"
		go mod tidy
	)
done
