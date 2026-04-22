module github.com/apet97/go-clockify

go 1.25.9

toolchain go1.25.9

replace github.com/apet97/go-clockify/internal/tracing/otel => ./internal/tracing/otel

require github.com/apet97/go-clockify/internal/tracing/otel v0.0.0-00010101000000-000000000000
