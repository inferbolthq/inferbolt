// Package campaigns gives optimization campaigns a durable, server-side home.
//
// A campaign used to run inside the CLI process: the plan, the trial history and
// the accumulated reasoning lived in memory and died with the terminal. Here the
// agent loop runs in the orchestrator as a River job, writing its progress to
// Postgres as it goes, so a campaign survives a disconnect, can be watched from a
// browser, and can be read back afterwards.
package campaigns

import (
	"time"

	"github.com/inferbolthq/inferbolt/internal/agent"
	"github.com/inferbolthq/inferbolt/internal/jobs"
)

type State string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// IsTerminal reports whether a campaign has finished, however it finished.
func IsTerminal(s State) bool {
	return s == StateCompleted || s == StateFailed || s == StateCancelled
}

// Campaign is one optimization run, from submitted goal to recommendation.
type Campaign struct {
	ID           string              `json:"id"`
	TenantID     string              `json:"tenant_id"`
	Goal         string              `json:"goal"`
	Model        string              `json:"model"`
	GPUProfile   string              `json:"gpu_profile"`
	Engines      []string            `json:"engines"`
	Workload     jobs.WorkloadConfig `json:"workload"`
	Budget       agent.Budget        `json:"budget"`
	PlannerModel string              `json:"planner_model"`
	State        State               `json:"state"`

	Recommendation *agent.Recommendation `json:"recommendation,omitempty"`
	Trials         []agent.Trial         `json:"trials,omitempty"`
	Usage          *agent.Usage          `json:"usage,omitempty"`
	ErrorMsg       string                `json:"error_msg,omitempty"`

	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ToAgentCampaign converts the stored record into the agent's input type.
func (c Campaign) ToAgentCampaign() agent.Campaign {
	return agent.Campaign{
		Goal:       c.Goal,
		Model:      c.Model,
		GPUProfile: c.GPUProfile,
		Workload:   c.Workload,
		Engines:    c.Engines,
		Budget:     c.Budget,
	}
}

// ToReport rebuilds the agent's report shape from a stored campaign, so the
// same renderer serves a live run and one read back later.
func (c Campaign) ToReport() *agent.Report {
	var elapsed time.Duration
	if c.StartedAt != nil {
		end := time.Now().UTC()
		if c.CompletedAt != nil {
			end = *c.CompletedAt
		}
		elapsed = end.Sub(*c.StartedAt)
	}
	var usage agent.Usage
	if c.Usage != nil {
		usage = *c.Usage
	}
	return &agent.Report{
		Campaign:       c.ToAgentCampaign(),
		Recommendation: c.Recommendation,
		Trials:         c.Trials,
		Usage:          usage,
		PlannerModel:   c.PlannerModel,
		Elapsed:        elapsed,
	}
}

// Event is one step a campaign took, as recorded for replay. Clients poll with
// an after-seq cursor, so a live view can reconnect and catch up rather than
// losing everything that happened while it was away.
type Event struct {
	Seq       int              `json:"seq"`
	Kind      string           `json:"kind"`
	Text      string           `json:"text,omitempty"`
	Trial     *agent.TrialSpec `json:"trial,omitempty"`
	Result    *jobs.Result     `json:"result,omitempty"`
	ElapsedMs int64            `json:"elapsed_ms"`
	CreatedAt time.Time        `json:"created_at"`
}
