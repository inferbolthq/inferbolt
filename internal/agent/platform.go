// Package agent runs goal-directed benchmark campaigns: it turns a plain-English
// optimization goal ("cheapest engine for this model under 200ms p99 TTFT") into
// a sequence of real benchmark jobs, reads the measurements back, and returns a
// recommendation grounded in what was actually measured.
//
// The model plans; it never measures. Every number in a report comes from a
// benchmark this package ran through the ordinary /v1/jobs API and read back out
// of TimescaleDB. The model's only side effect on the platform is submitting
// benchmark jobs, bounded by a Budget.
package agent

import (
	"context"
	"time"

	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/router"
)

// Platform is the slice of InferBolt the agent is allowed to act on. It is
// declared here, at the consumer, and satisfied by internal/cli's adapter over
// the ordinary authenticated gateway API — the agent has no privileged access
// path and no database handle of its own.
type Platform interface {
	// ListWorkers reports the registered workers, so a campaign can be planned
	// against hardware that actually exists.
	ListWorkers(ctx context.Context) ([]Worker, error)

	// ClassifyWorkload runs the deterministic rules-based router. It is a prior
	// on engine choice, not a substitute for measuring.
	ClassifyWorkload(ctx context.Context, in router.ClassificationInput) (router.ClassificationResult, error)

	// RunBenchmark submits one benchmark job and blocks until it reaches a
	// terminal state, returning the measured result.
	RunBenchmark(ctx context.Context, spec TrialSpec) (jobs.Result, error)

	// HistoricalResults returns past measurements for an (engine, model) pair,
	// so the agent can reuse a prior run instead of paying to repeat it.
	HistoricalResults(ctx context.Context, engine, model string, since time.Time) ([]jobs.Result, error)
}

// Worker is a registered benchmark worker.
type Worker struct {
	ID      string `json:"id"`
	GPUType string `json:"gpu_type"`
	Status  string `json:"status"`
}

// TrialSpec is one benchmark: a single engine and configuration, measured
// against the campaign's fixed model, GPU profile, and workload.
type TrialSpec struct {
	Model        string              `json:"model"`
	Engine       string              `json:"engine"`
	GPUProfile   string              `json:"gpu_profile"`
	Workload     jobs.WorkloadConfig `json:"workload"`
	EngineConfig jobs.EngineConfig   `json:"engine_config"`
}

// Trial pairs a spec with its outcome. A failed trial keeps its Err so the
// model can see what went wrong and adapt rather than retrying blindly.
type Trial struct {
	Spec     TrialSpec     `json:"spec"`
	Result   jobs.Result   `json:"result"`
	Err      string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`

	// Hypothesis is what the model expected to learn by spending this trial.
	Hypothesis string `json:"hypothesis,omitempty"`

	// Reused marks a trial answered from an earlier identical trial in the same
	// campaign rather than re-measured.
	Reused bool `json:"reused,omitempty"`
}

// OK reports whether the trial produced a usable measurement.
func (t Trial) OK() bool { return t.Err == "" && t.Result.TokPerSec > 0 }
