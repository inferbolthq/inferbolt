package campaigns

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/inferbolthq/inferbolt/internal/agent"
	"github.com/inferbolthq/inferbolt/internal/queue"
)

// runner is the campaign loop, injectable so the worker's lifecycle can be
// tested without a planner, an API key, or a network.
type runner interface {
	Run(ctx context.Context) (*agent.Report, error)
}

// RunnerFactory builds the loop for one campaign.
type RunnerFactory func(c Campaign, p agent.Platform, obs agent.Observer) (runner, error)

// campaignStore is what the worker needs from persistence, declared here so the
// lifecycle — claiming, completing, failing, cancelling — can be tested without
// a database. Satisfied by *Store.
type campaignStore interface {
	GetByID(ctx context.Context, id string) (*Campaign, error)
	MarkRunning(ctx context.Context, id string) (bool, error)
	Complete(ctx context.Context, id string, report *agent.Report) error
	Fail(ctx context.Context, id, msg string, report *agent.Report) error
	AppendEvent(ctx context.Context, campaignID string, e agent.Event) error
}

// Worker executes campaigns as River jobs inside the orchestrator.
type Worker struct {
	river.WorkerDefaults[queue.CampaignJobArgs]

	store   campaignStore
	deps    PlatformDeps
	factory RunnerFactory
	logger  *slog.Logger
}

// NewWorker wires the campaign runner. plannerModel is the Anthropic model used
// for planning; an empty string takes the agent's default.
func NewWorker(store campaignStore, deps PlatformDeps, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	w := &Worker{store: store, deps: deps, logger: logger}
	w.factory = func(c Campaign, p agent.Platform, obs agent.Observer) (runner, error) {
		return agent.New(p, c.ToAgentCampaign(),
			agent.WithModel(c.PlannerModel),
			agent.WithObserver(obs),
			agent.WithLogger(logger),
		)
	}
	return w
}

// Timeout gives the campaign its whole wall-clock budget plus slack for the
// final turn. The agent enforces the budget itself; this is the backstop.
func (w *Worker) Timeout(job *river.Job[queue.CampaignJobArgs]) time.Duration {
	d := time.Duration(job.Args.MaxDurationSec) * time.Second
	if d <= 0 {
		d = agent.DefaultBudget().MaxDuration
	}
	return d + 10*time.Minute
}

func (w *Worker) Work(ctx context.Context, job *river.Job[queue.CampaignJobArgs]) error {
	c, err := w.store.GetByID(ctx, job.Args.CampaignID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Deleted before it ran. Nothing to do, and retrying will not help.
			w.logger.Warn("campaign row missing; dropping job", "campaign_id", job.Args.CampaignID)
			return nil
		}
		return fmt.Errorf("load campaign %s: %w", job.Args.CampaignID, err)
	}

	// Claiming the row is what makes a River retry safe: a campaign already
	// running (or finished, or cancelled) must not get a second agent loop.
	claimed, err := w.store.MarkRunning(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("claim campaign %s: %w", c.ID, err)
	}
	if !claimed {
		w.logger.Info("campaign not claimable; skipping",
			"campaign_id", c.ID, "state", string(c.State))
		return nil
	}

	w.logger.Info("campaign started",
		"campaign_id", c.ID, "tenant_id", c.TenantID,
		"model", c.Model, "gpu_profile", c.GPUProfile, "max_trials", c.Budget.MaxTrials)

	platform := NewPlatform(c.ID, c.TenantID, w.deps)

	// Events are the campaign's visible progress. A failure to record one must
	// not take down a run that is otherwise working.
	observer := func(e agent.Event) {
		if err := w.store.AppendEvent(ctx, c.ID, e); err != nil {
			w.logger.Warn("could not record campaign event",
				"campaign_id", c.ID, "kind", string(e.Kind), "error", err)
		}
	}

	r, err := w.factory(*c, platform, observer)
	if err != nil {
		return w.finishFailed(ctx, c.ID, err, nil)
	}

	report, runErr := r.Run(ctx)
	if runErr != nil {
		if errors.Is(runErr, ErrCampaignCancelled) {
			w.logger.Info("campaign cancelled", "campaign_id", c.ID)
			// Complete() preserves the cancelled state and keeps the partial work.
			if err := w.store.Complete(ctx, c.ID, report); err != nil {
				return fmt.Errorf("record cancelled campaign %s: %w", c.ID, err)
			}
			return nil
		}
		return w.finishFailed(ctx, c.ID, runErr, report)
	}

	if err := w.store.Complete(ctx, c.ID, report); err != nil {
		return fmt.Errorf("record completed campaign %s: %w", c.ID, err)
	}
	w.logger.Info("campaign completed",
		"campaign_id", c.ID, "trials", len(report.Trials), "turns", report.Usage.Turns)
	return nil
}

// finishFailed records a terminal failure and returns nil: the campaign's
// outcome is the row, not the River job. Returning the error would have River
// retry a run whose budget is already spent.
func (w *Worker) finishFailed(ctx context.Context, id string, cause error, report *agent.Report) error {
	w.logger.Error("campaign failed", "campaign_id", id, "error", cause)
	if err := w.store.Fail(ctx, id, cause.Error(), report); err != nil {
		return fmt.Errorf("record failed campaign %s: %w", id, err)
	}
	return nil
}
