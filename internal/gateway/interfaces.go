package gateway

import (
	"context"
	"time"

	"github.com/inferbolthq/inferbolt/internal/campaigns"
	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/queue"
)

// JobStorer abstracts persistent job storage; satisfied by *config.Store.
type JobStorer interface {
	SaveJob(ctx context.Context, job jobs.Job) error
	GetJob(ctx context.Context, jobID, tenantID string) (*jobs.Job, error)
	ListJobs(ctx context.Context, tenantID, state string, limit, offset int) ([]jobs.Job, error)
	CountJobs(ctx context.Context, tenantID, state string) (int, error)
	UpdateJobState(ctx context.Context, jobID string, state jobs.JobState, errMsg string) error
}

// JobQueuer abstracts job enqueueing; satisfied by *queue.QueueClient.
type JobQueuer interface {
	Enqueue(ctx context.Context, args queue.BenchmarkJobArgs) error
	EnqueueCampaign(ctx context.Context, args queue.CampaignJobArgs) error
}

// CampaignStorer abstracts campaign persistence; satisfied by *campaigns.Store.
// Every method is tenant-scoped: the gateway never reads a campaign, or its
// events, without the caller's tenant.
type CampaignStorer interface {
	Create(ctx context.Context, c campaigns.Campaign) error
	Get(ctx context.Context, id, tenantID string) (*campaigns.Campaign, error)
	List(ctx context.Context, tenantID, state string, limit, offset int) ([]campaigns.Campaign, error)
	Count(ctx context.Context, tenantID, state string) (int, error)
	Cancel(ctx context.Context, id, tenantID string) error
	Events(ctx context.Context, campaignID string, afterSeq, limit int) ([]campaigns.Event, error)
}

// MetricsReader abstracts benchmark result queries; satisfied by *metrics.MetricsWriter.
//
// The engine/model query is deliberately the tenant-scoped one: the gateway
// serves authenticated tenants, and the unscoped variant that the collector and
// drift detector use reads across every tenant.
type MetricsReader interface {
	QueryByJob(ctx context.Context, jobID string) ([]jobs.Result, error)
	QueryByTenantEngineAndModel(ctx context.Context, tenantID, engine, model string, since time.Time) ([]jobs.Result, error)
}

// DBPinger abstracts a database ping; satisfied by *pgxpool.Pool.
type DBPinger interface {
	Ping(ctx context.Context) error
}
