package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("maggus")

func StartWorkItemSpan(ctx context.Context, itemID, title, repoURL string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "work_item",
		trace.WithAttributes(
			attribute.String("item.id", itemID),
			attribute.String("item.title", title),
			attribute.String("repo.url", repoURL),
		),
	)
}

func StartTaskSpan(ctx context.Context, taskID, title, agentName, model string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "task",
		trace.WithAttributes(
			attribute.String("task.id", taskID),
			attribute.String("task.title", title),
			attribute.String("agent.name", agentName),
			attribute.String("agent.model", model),
		),
	)
}

func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}
