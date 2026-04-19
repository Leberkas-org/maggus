package otel

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("maggus")

var (
	TasksTotal       metric.Int64Counter
	TasksActive      metric.Int64UpDownCounter
	QueueDepth       metric.Int64UpDownCounter
	QueuePending     metric.Int64UpDownCounter
	TokensInput      metric.Int64Counter
	TokensOutput     metric.Int64Counter
	TokensCacheRead  metric.Int64Counter
	TokensCacheCreate metric.Int64Counter
	CostUSD          metric.Float64Counter
	AgentDuration    metric.Float64Histogram
	TaskDuration     metric.Float64Histogram
	MergeDuration    metric.Float64Histogram
	MergeConflicts   metric.Int64Counter
)

func InitMetrics() error {
	var err error

	TasksTotal, err = meter.Int64Counter("maggus.tasks.total")
	if err != nil {
		return err
	}
	TasksActive, err = meter.Int64UpDownCounter("maggus.tasks.active")
	if err != nil {
		return err
	}
	QueueDepth, err = meter.Int64UpDownCounter("maggus.queue.depth")
	if err != nil {
		return err
	}
	QueuePending, err = meter.Int64UpDownCounter("maggus.queue.pending")
	if err != nil {
		return err
	}
	TokensInput, err = meter.Int64Counter("maggus.tokens.input")
	if err != nil {
		return err
	}
	TokensOutput, err = meter.Int64Counter("maggus.tokens.output")
	if err != nil {
		return err
	}
	TokensCacheRead, err = meter.Int64Counter("maggus.tokens.cache_read")
	if err != nil {
		return err
	}
	TokensCacheCreate, err = meter.Int64Counter("maggus.tokens.cache_creation")
	if err != nil {
		return err
	}
	CostUSD, err = meter.Float64Counter("maggus.cost.usd")
	if err != nil {
		return err
	}
	AgentDuration, err = meter.Float64Histogram("maggus.agent.duration_seconds")
	if err != nil {
		return err
	}
	TaskDuration, err = meter.Float64Histogram("maggus.task.duration_seconds")
	if err != nil {
		return err
	}
	MergeDuration, err = meter.Float64Histogram("maggus.git.merge.duration_seconds")
	if err != nil {
		return err
	}
	MergeConflicts, err = meter.Int64Counter("maggus.git.merge.conflicts")
	if err != nil {
		return err
	}

	return nil
}
