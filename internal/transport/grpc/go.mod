module github.com/apet97/go-clockify/internal/transport/grpc

go 1.25.10

replace github.com/apet97/go-clockify => ../../..

require (
	github.com/apet97/go-clockify v0.0.0
	google.golang.org/grpc v1.80.0
)

require (
	go.opentelemetry.io/otel/sdk/metric v1.43.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
