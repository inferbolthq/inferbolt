package campaigns

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/inferbolthq/inferbolt/internal/agent"
	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/queue"
	"github.com/inferbolthq/inferbolt/internal/router"
	"github.com/inferbolthq/inferbolt/internal/workers"
)

// ErrCampaignCancelled stops a run when the campaign was cancelled while it was
// in flight. It is checked between trials rather than mid-benchmark, so a
// cancellation never orphans a running engine.
var ErrCampaignCancelled = errors.New("campaign cancelled")

// Dependencies the in-process platform needs, declared here at the consumer.
type (
	JobStore interface {
		SaveJob(ctx context.Context, job jobs.Job) error
		GetJobByID(ctx context.Context, jobID string) (*jobs.Job, error)
	}
	JobQueuer interface {
		Enqueue(ctx context.Context, args queue.BenchmarkJobArgs) error
	}
	MetricsReader interface {
		QueryByJob(ctx context.Context, jobID string) ([]jobs.Result, error)
		QueryByTenantEngineAndModel(ctx context.Context, tenantID, engine, model string, since time.Time) ([]jobs.Result, error)
	}
	WorkerLister interface {
		List() []workers.WorkerEntry
	}
	StateReader interface {
		State(ctx context.Context, id string) (State, error)
	}
)

// Platform is agent.Platform implemented inside the orchestrator, against the
// store and queue directly rather than over HTTP.
//
// It is not a shortcut around authorization: the campaign's tenant is fixed at
// construction from the stored row, every job it creates is written with that
// tenant, and its historical reads go through the tenant-scoped metrics query.
// The agent cannot widen its own scope, because nothing it returns feeds back
// into which tenant this platform acts as.
type Platform struct {
	campaignID string
	tenantID   string

	jobs     JobStore
	queue    JobQueuer
	metrics  MetricsReader
	registry WorkerLister
	state    StateReader

	pollInterval time.Duration
	jobTimeout   time.Duration
}

// PlatformDeps bundles what NewPlatform needs from the orchestrator.
type PlatformDeps struct {
	Jobs     JobStore
	Queue    JobQueuer
	Metrics  MetricsReader
	Registry WorkerLister
	State    StateReader

	// PollInterval defaults to 3s, JobTimeout to 60m — the same ceiling the
	// River benchmark worker enforces on a single job.
	PollInterval time.Duration
	JobTimeout   time.Duration
}

func NewPlatform(campaignID, tenantID string, d PlatformDeps) *Platform {
	if d.PollInterval <= 0 {
		d.PollInterval = 3 * time.Second
	}
	if d.JobTimeout <= 0 {
		d.JobTimeout = 60 * time.Minute
	}
	return &Platform{
		campaignID:   campaignID,
		tenantID:     tenantID,
		jobs:         d.Jobs,
		queue:        d.Queue,
		metrics:      d.Metrics,
		registry:     d.Registry,
		state:        d.State,
		pollInterval: d.PollInterval,
		jobTimeout:   d.JobTimeout,
	}
}

func (p *Platform) ListWorkers(context.Context) ([]agent.Worker, error) {
	entries := p.registry.List()
	out := make([]agent.Worker, 0, len(entries))
	for _, e := range entries {
		out = append(out, agent.Worker{ID: e.ID, GPUType: e.GPUType, Status: string(e.Status)})
	}
	return out, nil
}

func (p *Platform) ClassifyWorkload(_ context.Context, in router.ClassificationInput) (router.ClassificationResult, error) {
	return router.Classify(in), nil
}

func (p *Platform) HistoricalResults(ctx context.Context, engine, model string, since time.Time) ([]jobs.Result, error) {
	return p.metrics.QueryByTenantEngineAndModel(ctx, p.tenantID, engine, model, since)
}

// RunBenchmark submits one trial and blocks until the job reaches a terminal
// state. It is the same pipeline a CLI user's job goes through — created,
// enqueued on River, dispatched to a worker — so a campaign's benchmarks are
// ordinary jobs, visible and cancellable like any other.
func (p *Platform) RunBenchmark(ctx context.Context, spec agent.TrialSpec) (jobs.Result, error) {
	if err := p.checkCancelled(ctx); err != nil {
		return jobs.Result{}, err
	}

	// The planner's arguments are schema-constrained, but this is a second
	// entry point into the job pipeline and validates like the first one.
	if err := jobs.ValidateEngines([]string{spec.Engine}); err != nil {
		return jobs.Result{}, err
	}
	if err := jobs.ValidateWorkload(spec.Workload); err != nil {
		return jobs.Result{}, err
	}
	if err := jobs.ValidateEngineConfig(spec.EngineConfig); err != nil {
		return jobs.Result{}, err
	}

	now := time.Now().UTC()
	job := jobs.Job{
		ID:             jobs.NewID(),
		TenantID:       p.tenantID,
		Model:          spec.Model,
		Engines:        []string{spec.Engine},
		WorkloadConfig: spec.Workload,
		EngineConfig:   spec.EngineConfig,
		GPUProfile:     spec.GPUProfile,
		State:          jobs.StatePending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := p.jobs.SaveJob(ctx, job); err != nil {
		return jobs.Result{}, fmt.Errorf("save benchmark job: %w", err)
	}
	if err := p.queue.Enqueue(ctx, queue.BenchmarkJobArgs{
		JobID:        job.ID,
		Model:        job.Model,
		Engines:      job.Engines,
		Workload:     queue.WorkloadConfig(job.WorkloadConfig),
		EngineConfig: queue.EngineConfig(job.EngineConfig),
		GPUProfile:   job.GPUProfile,
		TenantID:     job.TenantID,
	}); err != nil {
		return jobs.Result{}, fmt.Errorf("enqueue benchmark job: %w", err)
	}

	final, err := p.waitForJob(ctx, job.ID)
	if err != nil {
		return jobs.Result{}, err
	}
	if final.State != jobs.StateCompleted {
		msg := final.ErrorMsg
		if msg == "" {
			msg = "no error message recorded"
		}
		return jobs.Result{}, fmt.Errorf("job %s ended %s: %s", job.ID, final.State, msg)
	}

	results, err := p.metrics.QueryByJob(ctx, job.ID)
	if err != nil {
		return jobs.Result{}, fmt.Errorf("read results for job %s: %w", job.ID, err)
	}
	for _, r := range results {
		if r.Engine == spec.Engine {
			return r, nil
		}
	}
	return jobs.Result{}, fmt.Errorf("job %s completed but produced no %s result", job.ID, spec.Engine)
}

func (p *Platform) waitForJob(ctx context.Context, jobID string) (*jobs.Job, error) {
	deadline := time.NewTimer(p.jobTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, fmt.Errorf("job %s did not finish within %s", jobID, p.jobTimeout)
		case <-ticker.C:
			job, err := p.jobs.GetJobByID(ctx, jobID)
			if err != nil {
				continue // transient read error; try again on the next tick
			}
			if jobs.IsTerminal(job.State) {
				return job, nil
			}
		}
	}
}

func (p *Platform) checkCancelled(ctx context.Context) error {
	st, err := p.state.State(ctx, p.campaignID)
	if err != nil {
		return nil // a read failure should not abort a campaign mid-flight
	}
	if st == StateCancelled {
		return ErrCampaignCancelled
	}
	return nil
}
