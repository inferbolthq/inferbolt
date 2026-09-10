package agent

import (
	"time"

	"github.com/inferbolthq/inferbolt/internal/jobs"
)

// EventKind identifies what a campaign just did. The CLI renders these as they
// arrive; a campaign against real hardware runs for many minutes, so silence
// until the final report is not an acceptable UX.
type EventKind string

const (
	EventPlan        EventKind = "plan"   // the model said something out loud
	EventThinking    EventKind = "think"  // summarized reasoning between steps
	EventTrialStart  EventKind = "trial"  // a benchmark job is being submitted
	EventTrialDone   EventKind = "result" // a benchmark returned measurements
	EventTrialFailed EventKind = "failed" // a benchmark errored; the campaign continues
	EventLookup      EventKind = "lookup" // a read-only tool call
	EventWarn        EventKind = "warn"   // budget pressure, degraded behaviour
	EventDone        EventKind = "done"   // a recommendation was submitted
)

// Event is one step of a campaign.
type Event struct {
	Kind EventKind
	Text string

	// Trial is set on trial events; Result is set on EventTrialDone.
	Trial  *TrialSpec
	Result *jobs.Result

	Elapsed time.Duration
}

// Observer receives events as a campaign runs. It is called synchronously from
// the campaign goroutine, so implementations should not block.
type Observer func(Event)

func (a *Agent) emit(e Event) {
	if a.observer == nil {
		return
	}
	e.Elapsed = a.spend.elapsed()
	a.observer(e)
}
