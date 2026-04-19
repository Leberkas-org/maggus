package daemon

type EventType int

const (
	EventPlanDetected EventType = iota
	EventItemEnqueued
	EventItemApproved
	EventItemSkipped
	EventTaskStarted
	EventTaskCompleted
	EventTaskFailed
	EventItemCompleted
	EventStopRequested
)

type Event struct {
	Type   EventType
	ItemID string
	TaskID string
	Error  error
}
