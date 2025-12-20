package tracing

import (
	"context"
	"os"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Init sets up global tracer provider and propagator. Returns shutdown func.
func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	endpoint := getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	insecureFlag := getenvBool("OTEL_EXPORTER_OTLP_INSECURE", true)

	clientOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
	if insecureFlag {
		clientOpts = append(clientOpts, otlptracegrpc.WithInsecure())
	} else {
		clientOpts = append(clientOpts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))))
	}

	exp, err := otlptracegrpc.New(ctx, clientOpts...)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp.Shutdown, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}
