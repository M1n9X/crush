package pubsub

import "context"

const (
	CreatedEvent EventType = "created"
	UpdatedEvent EventType = "updated"
	DeletedEvent EventType = "deleted"

	// Workflow lifecycle events
	WorkflowStartedEvent   EventType = "workflow_started"
	WorkflowPausedEvent    EventType = "workflow_paused"
	WorkflowResumedEvent   EventType = "workflow_resumed"
	WorkflowCompletedEvent EventType = "workflow_completed"
	WorkflowFailedEvent    EventType = "workflow_failed"

	// Step lifecycle events
	StepStartedEvent   EventType = "step_started"
	StepCompletedEvent EventType = "step_completed"
	StepFailedEvent    EventType = "step_failed"
	StepWaitingEvent   EventType = "step_waiting"

	// Approval events
	ApprovalRequestedEvent EventType = "approval_requested"
	ApprovalGrantedEvent   EventType = "approval_granted"
	ApprovalDeniedEvent    EventType = "approval_denied"
)

type Suscriber[T any] interface {
	Subscribe(context.Context) <-chan Event[T]
}

type (
	// EventType identifies the type of event
	EventType string

	// Event represents an event in the lifecycle of a resource
	Event[T any] struct {
		Type    EventType
		Payload T
	}

	Publisher[T any] interface {
		Publish(EventType, T)
	}
)

// UpdateAvailableMsg is sent when a new version is available.
type UpdateAvailableMsg struct {
	CurrentVersion string
	LatestVersion  string
	IsDevelopment  bool
}
