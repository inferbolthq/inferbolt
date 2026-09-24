package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// WorkloadConfig mirrors jobs.WorkloadConfig for River queue serialization.
// Defined here to avoid an import cycle between the jobs and queue packages.
type WorkloadConfig struct {
	Concurrency  int `json:"concurrency"`
	PromptTokens int `json:"prompt_tokens"`
	OutputTokens int `json:"output_tokens"`
	NumRequests  int `json:"num_requests"`
}

// EngineConfig mirrors jobs.EngineConfig, for the same reason WorkloadConfig does.
type EngineConfig struct {
	Quantization         string  `json:"quantization,omitempty"`
	TensorParallel       int     `json:"tensor_parallel,omitempty"`
	MaxBatchSize         int     `json:"max_batch_size,omitempty"`
	MaxModelLen          int     `json:"max_model_len,omitempty"`
	GPUMemoryUtilization float64 `json:"gpu_memory_utilization,omitempty"`
}

type BenchmarkJobArgs struct {
	JobID        string         `json:"job_id"`
	Model        string         `json:"model"`
	Engines      []string       `json:"engines"`
	Workload     WorkloadConfig `json:"workload"`
	EngineConfig EngineConfig   `json:"engine_config"`
	GPUProfile   string         `json:"gpu_profile"`
	TenantID     string         `json:"tenant_id"`
}

func (BenchmarkJobArgs) Kind() string { return "benchmark_job" }

// CampaignJobArgs runs one optimization campaign. The row in public.campaigns
// holds the detail; only what River needs to schedule and time-box it lives here.
type CampaignJobArgs struct {
	CampaignID     string `json:"campaign_id"`
	TenantID       string `json:"tenant_id"`
	MaxDurationSec int    `json:"max_duration_sec"`
}

func (CampaignJobArgs) Kind() string { return "campaign_job" }

type QueueClient struct {
	client *river.Client[pgx.Tx]
	pool   *pgxpool.Pool
}

// NewQueueClient runs River schema migrations and returns an insert-ready client.
// Workers are registered later via Start.
func NewQueueClient(ctx context.Context, pool *pgxpool.Pool) (*QueueClient, error) {
	driver := riverpgxv5.New(pool)

	migrator := rivermigrate.New(driver, nil)
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return nil, fmt.Errorf("river migrate up: %w", err)
	}

	// Insert-only client — no workers configured until Start is called.
	rc, err := river.NewClient[pgx.Tx](driver, &river.Config{})
	if err != nil {
		return nil, fmt.Errorf("new river client: %w", err)
	}

	return &QueueClient{client: rc, pool: pool}, nil
}

func (q *QueueClient) Enqueue(ctx context.Context, args BenchmarkJobArgs) error {
	_, err := q.client.Insert(ctx, args, &river.InsertOpts{
		MaxAttempts: 3,
		Queue:       "benchmarks",
	})
	if err != nil {
		return fmt.Errorf("enqueue benchmark job: %w", err)
	}
	return nil
}

// EnqueueCampaign schedules a campaign on its own queue. Campaigns occupy a
// worker slot for as long as they run — hours, potentially — so they must not
// share a pool with benchmark dispatch, which would starve it.
//
// MaxAttempts is 1: a campaign that failed has already spent its trial budget
// and its planner tokens, and re-running it would spend them again.
func (q *QueueClient) EnqueueCampaign(ctx context.Context, args CampaignJobArgs) error {
	_, err := q.client.Insert(ctx, args, &river.InsertOpts{
		MaxAttempts: 1,
		Queue:       "campaigns",
	})
	if err != nil {
		return fmt.Errorf("enqueue campaign job: %w", err)
	}
	return nil
}

// Start creates a worker-enabled River client and begins processing jobs.
// It blocks until ctx is cancelled.
func (q *QueueClient) Start(ctx context.Context, workers *river.Workers) error {
	rc, err := river.NewClient[pgx.Tx](riverpgxv5.New(q.pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			"benchmarks": {MaxWorkers: 10},
			"campaigns":  {MaxWorkers: 4},
		},
		Workers: workers,
	})
	if err != nil {
		return fmt.Errorf("new river worker client: %w", err)
	}
	q.client = rc
	if err = rc.Start(ctx); err != nil {
		return fmt.Errorf("river start: %w", err)
	}
	<-ctx.Done()
	// Deliberately not derived from ctx: it is already cancelled by the time we
	// get here, so a derived context would make graceful shutdown return at once
	// and kill in-flight jobs.
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	return rc.Stop(stopCtx) //nolint:contextcheck // see above

}
