package engine

import (
	"time"

	"github.com/zclconf/go-cty/cty"
)

type EventStatus string

const (
	StatusLevelStarted  EventStatus = "level_started"
	StatusLevelDone     EventStatus = "level_done"
	StatusStepStarted   EventStatus = "step_started"
	StatusStepSucceeded EventStatus = "step_succeeded"
	StatusStepFailed    EventStatus = "step_failed"
	StatusStepSkipped   EventStatus = "step_skipped"
	StatusStepCancelled EventStatus = "step_cancelled"
)

type Event struct {
	Workflow  string
	Level     int
	StepId    string
	StepType  string
	Status    EventStatus
	Err       error
	Outputs   map[string]cty.Value
	Timestamp time.Time
}

type Reporter interface {
	Emit(Event)
}

type ChanReporter struct {
	events chan Event
}

func NewChanReporter(buffer int) *ChanReporter {
	return &ChanReporter{events: make(chan Event, buffer)}
}

func (r *ChanReporter) Emit(e Event) {
	e.Timestamp = time.Now()
	select {
	case r.events <- e:
	default:
	}
}

func (r *ChanReporter) Events() <-chan Event {
	return r.events
}

func (r *ChanReporter) Close() {
	close(r.events)
}
