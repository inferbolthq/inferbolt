package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/inferbolthq/inferbolt/internal/agent"
	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/router"
)

// AgentPlatform adapts the gateway client to agent.Platform.
//
// The agent is an ordinary API client: it authenticates with the same bearer
// token, hits the same tenant-scoped endpoints, and is subject to the same rate
// limits as a human running the CLI. It has no database handle and no path to
// the orchestrator's internal port.
type AgentPlatform struct {
	client *Client

	// OnJobState, when set, is called on every poll of a running benchmark so
	// the CLI can show progress during the minutes a trial takes.
	OnJobState func(jobID string, state jobs.JobState)
}

// NewAgentPlatform adapts c for use by the agent.
func NewAgentPlatform(c *Client) *AgentPlatform {
	return &AgentPlatform{client: c}
}

func (p *AgentPlatform) ListWorkers(ctx context.Context) ([]agent.Worker, error) {
	entries, err := p.client.ListWorkers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]agent.Worker, 0, len(entries))
	for _, e := range entries {
		out = append(out, agent.Worker{
			ID:      e.ID,
			GPUType: e.GPUType,
			Status:  string(e.Status),
		})
	}
	return out, nil
}

func (p *AgentPlatform) ClassifyWorkload(ctx context.Context, in router.ClassificationInput) (router.ClassificationResult, error) {
	result, err := p.client.ClassifyWorkload(ctx, in)
	if err != nil {
		return router.ClassificationResult{}, err
	}
	return *result, nil
}

func (p *AgentPlatform) HistoricalResults(ctx context.Context, engine, model string, since time.Time) ([]jobs.Result, error) {
	return p.client.GetMetrics(ctx, engine, model, since)
}

// RunBenchmark submits one trial and blocks until it reaches a terminal state.
func (p *AgentPlatform) RunBenchmark(ctx context.Context, spec agent.TrialSpec) (jobs.Result, error) {
	resp, err := p.client.CreateJob(ctx, CreateJobRequest{
		Model:        spec.Model,
		Engines:      []string{spec.Engine},
		Workload:     spec.Workload,
		EngineConfig: spec.EngineConfig,
		GPUProfile:   spec.GPUProfile,
	})
	if err != nil {
		return jobs.Result{}, fmt.Errorf("submit benchmark: %w", err)
	}

	var onUpdate func(jobs.Job)
	if p.OnJobState != nil {
		onUpdate = func(j jobs.Job) { p.OnJobState(j.ID, j.State) }
	}

	job, err := p.client.PollJob(ctx, resp.JobID, onUpdate)
	if err != nil {
		return jobs.Result{}, fmt.Errorf("job %s: %w", resp.JobID, err)
	}
	if job.State != jobs.StateCompleted {
		msg := job.ErrorMsg
		if msg == "" {
			msg = "no error message recorded"
		}
		return jobs.Result{}, fmt.Errorf("job %s ended %s: %s", resp.JobID, job.State, msg)
	}

	results, err := p.client.GetJobResults(ctx, resp.JobID)
	if err != nil {
		return jobs.Result{}, fmt.Errorf("read results for job %s: %w", resp.JobID, err)
	}
	for _, r := range results {
		if r.Engine == spec.Engine {
			return r, nil
		}
	}
	// A completed job with no matching row means results were lost between the
	// worker and TimescaleDB; reporting it as a failed trial is honest, and the
	// agent can decide whether to retry.
	return jobs.Result{}, fmt.Errorf("job %s completed but produced no %s result", resp.JobID, spec.Engine)
}
