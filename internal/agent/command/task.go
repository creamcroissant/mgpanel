package command

import (
	"context"
	"time"
)

const (
	EventTypeAccepted = "accepted"
	EventTypeProgress = "progress"
	EventTypeResult   = "result"

	StatusClaimed           = "claimed"
	StatusInProgress        = "in_progress"
	StatusSuccess           = "success"
	StatusFailed            = "failed"
	StatusTimeout           = "timeout"
	StatusCancelled         = "cancelled"
	StatusUnsupportedAction = "unsupported_action"
	StatusQueueFull         = "queue_full"

	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

type Task struct {
	ID             string
	OperationType  string
	RequestPayload []byte
	Timeout        time.Duration
	Source         string
	CorrelationID  string
	CreatedAt      int64
	UpdatedAt      int64
}

type Result struct {
	Status       string
	Phase        string
	Level        string
	Message      string
	Payload      []byte
	ErrorMessage string
	Terminal     bool
}

type Event struct {
	CommandID     string
	OperationType string
	EventType     string
	Status        string
	Phase         string
	Level         string
	Message       string
	Payload       []byte
	ErrorMessage  string
	OccurredAt    int64
	Sequence      int64
	Terminal      bool
}

type Reporter interface {
	Report(ctx context.Context, event Event) error
}

type Handler func(ctx context.Context, task Task, reporter Reporter) Result
