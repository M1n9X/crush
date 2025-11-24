package telemetry

import (
	"context"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
)

// EventType identifies the kind of telemetry event.
type EventType string

const (
	EventTokensUsed   EventType = "tokens_used"
	EventToolFinished EventType = "tool_finished"
)

// Event is a lightweight envelope for telemetry consumers.
type Event struct {
	Type      EventType
	SessionID string
	ToolName  string
	Duration  time.Duration
	Cost      float64
	TokensIn  int64
	TokensOut int64
	At        time.Time
}

// Recorder publishes telemetry events and allows subscribers to listen in.
type Recorder struct {
	broker *pubsub.Broker[Event]
}

// NewRecorder creates a new recorder with an internal pub/sub broker.
func NewRecorder() *Recorder {
	return &Recorder{
		broker: pubsub.NewBroker[Event](),
	}
}

// Publish emits an event to all subscribers.
func (r *Recorder) Publish(ev Event) {
	if r == nil || r.broker == nil {
		return
	}
	r.broker.Publish(pubsub.CreatedEvent, ev)
}

// Subscribe returns a channel that receives telemetry events.
func (r *Recorder) Subscribe(ctx context.Context) <-chan pubsub.Event[Event] {
	if r == nil || r.broker == nil {
		return nil
	}
	return r.broker.Subscribe(ctx)
}
