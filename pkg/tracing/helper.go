package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

func Tracer() trace.Tracer {
	return otel.Tracer("sms-gateway")
}

func Attr(key, val string) attribute.KeyValue {
	return attribute.String(key, val)
}

func WithAttributes(attrs ...attribute.KeyValue) trace.SpanStartOption {
	return trace.WithAttributes(attrs...)
}

func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

func WithUser(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	member, err := baggage.NewMember("user.id", userID)
	if err != nil {
		return ctx
	}
	bag, err := baggage.New(member)
	if err != nil {
		return ctx
	}
	return baggage.ContextWithBaggage(ctx, bag)
}

func UserIDFromContext(ctx context.Context) string {
	bag := baggage.FromContext(ctx)
	return bag.Member("user.id").Value()
}
