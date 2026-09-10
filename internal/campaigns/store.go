package campaigns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inferbolthq/inferbolt/internal/agent"
)

// ErrNotFound is returned when no campaign matches the id (and tenant, where
// the caller is tenant-scoped). Callers should surface this as 404 rather than
// 403 — telling an attacker that someone else's campaign exists is a leak.
var ErrNotFound = errors.New("campaign not found")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const campaignColumns = `id, tenant_id, goal, model, gpu_profile, engines, workload, budget,
	planner_model, state, recommendation, trials, usage, COALESCE(error_msg, ''),
	created_at, updated_at, started_at, completed_at`

func (s *Store) Create(ctx context.Context, c Campaign) error {
	workload, err := json.Marshal(c.Workload)
	if err != nil {
		return fmt.Errorf("marshal workload: %w", err)
	}
	budget, err := json.Marshal(c.Budget)
	if err != nil {
		return fmt.Errorf("marshal budget: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO public.campaigns
		  (id, tenant_id, goal, model, gpu_profile, engines, workload, budget,
		   planner_model, state, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)`,
		c.ID, c.TenantID, c.Goal, c.Model, c.GPUProfile, c.Engines,
		workload, budget, c.PlannerModel, string(c.State), c.CreatedAt)
	if err != nil {
		return fmt.Errorf("create campaign: %w", err)
	}
	return nil
}

// Get reads a campaign scoped to its owner.
func (s *Store) Get(ctx context.Context, id, tenantID string) (*Campaign, error) {
	return s.queryOne(ctx,
		"SELECT "+campaignColumns+" FROM public.campaigns WHERE id = $1 AND tenant_id = $2",
		id, tenantID)
}

// GetByID reads a campaign without a tenant filter. For the in-process runner
// only, which is acting on behalf of the campaign's own tenant — never reachable
// from a request handler.
func (s *Store) GetByID(ctx context.Context, id string) (*Campaign, error) {
	return s.queryOne(ctx,
		"SELECT "+campaignColumns+" FROM public.campaigns WHERE id = $1", id)
}

func (s *Store) queryOne(ctx context.Context, query string, args ...any) (*Campaign, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query campaign: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("query campaign: %w", err)
		}
		return nil, ErrNotFound
	}
	c, err := scanCampaign(rows)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) List(ctx context.Context, tenantID, state string, limit, offset int) ([]Campaign, error) {
	query := "SELECT " + campaignColumns + " FROM public.campaigns WHERE tenant_id = $1"
	args := []any{tenantID}
	if state != "" {
		query += " AND state = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4"
		args = append(args, state, limit, offset)
	} else {
		query += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"
		args = append(args, limit, offset)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()

	out := []Campaign{}
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Count(ctx context.Context, tenantID, state string) (int, error) {
	query := "SELECT COUNT(*) FROM public.campaigns WHERE tenant_id = $1"
	args := []any{tenantID}
	if state != "" {
		query += " AND state = $2"
		args = append(args, state)
	}
	var n int
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count campaigns: %w", err)
	}
	return n, nil
}

// MarkRunning claims a pending campaign. It reports whether the claim succeeded,
// so a River retry of an already-running campaign does not start a second agent
// loop against the same row.
func (s *Store) MarkRunning(ctx context.Context, id string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE public.campaigns
		SET state = $1, started_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND state = $3`,
		string(StateRunning), id, string(StatePending))
	if err != nil {
		return false, fmt.Errorf("mark campaign running: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Complete stores the finished report. A campaign cancelled mid-flight keeps its
// cancelled state — the work it managed to do is still recorded.
func (s *Store) Complete(ctx context.Context, id string, report *agent.Report) error {
	var rec, trials, usage []byte
	var err error
	if report != nil {
		if rec, err = json.Marshal(report.Recommendation); err != nil {
			return fmt.Errorf("marshal recommendation: %w", err)
		}
		if trials, err = json.Marshal(report.Trials); err != nil {
			return fmt.Errorf("marshal trials: %w", err)
		}
		if usage, err = json.Marshal(report.Usage); err != nil {
			return fmt.Errorf("marshal usage: %w", err)
		}
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE public.campaigns
		SET state = CASE WHEN state = $5 THEN $5 ELSE $1 END,
		    recommendation = $2, trials = $3, usage = $4,
		    completed_at = NOW(), updated_at = NOW()
		WHERE id = $6`,
		string(StateCompleted), rec, trials, usage, string(StateCancelled), id)
	if err != nil {
		return fmt.Errorf("complete campaign: %w", err)
	}
	return nil
}

// Fail records a terminal failure, keeping whatever partial work was done.
func (s *Store) Fail(ctx context.Context, id, msg string, report *agent.Report) error {
	var trials, usage []byte
	if report != nil {
		trials, _ = json.Marshal(report.Trials) //nolint:errcheck // best effort; the failure message matters more
		usage, _ = json.Marshal(report.Usage)   //nolint:errcheck
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE public.campaigns
		SET state = CASE WHEN state = $4 THEN $4 ELSE $1 END,
		    error_msg = $2, trials = COALESCE($5, trials), usage = COALESCE($6, usage),
		    completed_at = NOW(), updated_at = NOW()
		WHERE id = $3`,
		string(StateFailed), msg, id, string(StateCancelled), trials, usage)
	if err != nil {
		return fmt.Errorf("fail campaign: %w", err)
	}
	return nil
}

// Cancel marks a non-terminal campaign cancelled. The running loop notices
// before its next trial rather than being killed mid-benchmark.
func (s *Store) Cancel(ctx context.Context, id, tenantID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE public.campaigns
		SET state = $1, updated_at = NOW()
		WHERE id = $2 AND tenant_id = $3 AND state IN ($4, $5)`,
		string(StateCancelled), id, tenantID, string(StatePending), string(StateRunning))
	if err != nil {
		return fmt.Errorf("cancel campaign: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// State reads just the current state, for the cancellation check between trials.
func (s *Store) State(ctx context.Context, id string) (State, error) {
	var st string
	if err := s.pool.QueryRow(ctx,
		"SELECT state FROM public.campaigns WHERE id = $1", id).Scan(&st); err != nil {
		return "", fmt.Errorf("read campaign state: %w", err)
	}
	return State(st), nil
}

// AppendEvent records one step. seq is assigned in the same statement; the
// campaign's runner is the only writer, so there is no contention on it.
func (s *Store) AppendEvent(ctx context.Context, campaignID string, e agent.Event) error {
	var trial, result []byte
	var err error
	if e.Trial != nil {
		if trial, err = json.Marshal(e.Trial); err != nil {
			return fmt.Errorf("marshal event trial: %w", err)
		}
	}
	if e.Result != nil {
		if result, err = json.Marshal(e.Result); err != nil {
			return fmt.Errorf("marshal event result: %w", err)
		}
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO public.campaign_events (campaign_id, seq, kind, text, trial, result, elapsed_ms)
		VALUES ($1,
		        (SELECT COALESCE(MAX(seq), 0) + 1 FROM public.campaign_events WHERE campaign_id = $1),
		        $2, $3, $4, $5, $6)`,
		campaignID, string(e.Kind), e.Text, trial, result, e.Elapsed.Milliseconds())
	if err != nil {
		return fmt.Errorf("append campaign event: %w", err)
	}
	return nil
}

// Events returns steps after afterSeq, oldest first.
func (s *Store) Events(ctx context.Context, campaignID string, afterSeq, limit int) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT seq, kind, COALESCE(text, ''), trial, result, elapsed_ms, created_at
		FROM public.campaign_events
		WHERE campaign_id = $1 AND seq > $2
		ORDER BY seq ASC
		LIMIT $3`, campaignID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("read campaign events: %w", err)
	}
	defer rows.Close()

	out := []Event{}
	for rows.Next() {
		var e Event
		var trial, result []byte
		if err := rows.Scan(&e.Seq, &e.Kind, &e.Text, &trial, &result, &e.ElapsedMs, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan campaign event: %w", err)
		}
		if len(trial) > 0 {
			if err := json.Unmarshal(trial, &e.Trial); err != nil {
				return nil, fmt.Errorf("unmarshal event trial: %w", err)
			}
		}
		if len(result) > 0 {
			if err := json.Unmarshal(result, &e.Result); err != nil {
				return nil, fmt.Errorf("unmarshal event result: %w", err)
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanCampaign(rows pgx.Rows) (Campaign, error) {
	var c Campaign
	var state string
	var workload, budget, rec, trials, usage []byte
	var startedAt, completedAt *time.Time

	if err := rows.Scan(
		&c.ID, &c.TenantID, &c.Goal, &c.Model, &c.GPUProfile, &c.Engines,
		&workload, &budget, &c.PlannerModel, &state, &rec, &trials, &usage,
		&c.ErrorMsg, &c.CreatedAt, &c.UpdatedAt, &startedAt, &completedAt,
	); err != nil {
		return c, fmt.Errorf("scan campaign: %w", err)
	}

	c.State = State(state)
	c.StartedAt, c.CompletedAt = startedAt, completedAt

	if err := json.Unmarshal(workload, &c.Workload); err != nil {
		return c, fmt.Errorf("unmarshal workload: %w", err)
	}
	if err := json.Unmarshal(budget, &c.Budget); err != nil {
		return c, fmt.Errorf("unmarshal budget: %w", err)
	}
	for _, f := range []struct {
		raw []byte
		dst any
	}{{rec, &c.Recommendation}, {trials, &c.Trials}, {usage, &c.Usage}} {
		if len(f.raw) == 0 {
			continue
		}
		if err := json.Unmarshal(f.raw, f.dst); err != nil {
			return c, fmt.Errorf("unmarshal campaign result field: %w", err)
		}
	}
	return c, nil
}
