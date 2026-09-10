package campaigns

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/inferbolthq/inferbolt/internal/agent"
	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/queue"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeStore struct {
	campaign   *Campaign
	getErr     error
	claimable  bool
	claimCalls int

	completed     bool
	completedWith *agent.Report
	failedMsg     string
	events        []agent.Event
}

func (f *fakeStore) GetByID(context.Context, string) (*Campaign, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.campaign, nil
}

func (f *fakeStore) MarkRunning(context.Context, string) (bool, error) {
	f.claimCalls++
	return f.claimable, nil
}

func (f *fakeStore) Complete(_ context.Context, _ string, r *agent.Report) error {
	f.completed = true
	f.completedWith = r
	return nil
}

func (f *fakeStore) Fail(_ context.Context, _, msg string, _ *agent.Report) error {
	f.failedMsg = msg
	return nil
}

func (f *fakeStore) AppendEvent(_ context.Context, _ string, e agent.Event) error {
	f.events = append(f.events, e)
	return nil
}

type fakeRunner struct {
	report *agent.Report
	err    error
	emit   []agent.Event
	obs    agent.Observer
}

func (f *fakeRunner) Run(context.Context) (*agent.Report, error) {
	for _, e := range f.emit {
		f.obs(e)
	}
	return f.report, f.err
}

func testCampaignRow() *Campaign {
	return &Campaign{
		ID:         "camp-1",
		TenantID:   "tenant-a",
		Goal:       "cheapest engine",
		Model:      "meta-llama/Llama-3.1-8B",
		GPUProfile: "a100-80gb",
		Engines:    []string{"vllm"},
		Workload:   jobs.WorkloadConfig{Concurrency: 32, PromptTokens: 512, OutputTokens: 256, NumRequests: 200},
		Budget:     agent.Budget{MaxTrials: 3, MaxDuration: time.Hour, MaxTurns: 10},
		State:      StatePending,
	}
}

func newTestWorker(store campaignStore, r *fakeRunner) *Worker {
	w := NewWorker(store, PlatformDeps{}, nil)
	w.factory = func(_ Campaign, _ agent.Platform, obs agent.Observer) (runner, error) {
		r.obs = obs
		return r, nil
	}
	return w
}

func riverJob(id string, maxDurationSec int) *river.Job[queue.CampaignJobArgs] {
	return &river.Job[queue.CampaignJobArgs]{
		JobRow: &rivertype.JobRow{},
		Args:   queue.CampaignJobArgs{CampaignID: id, TenantID: "tenant-a", MaxDurationSec: maxDurationSec},
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestWorker_RecordsACompletedCampaign(t *testing.T) {
	store := &fakeStore{campaign: testCampaignRow(), claimable: true}
	report := &agent.Report{
		Recommendation: &agent.Recommendation{Engine: "vllm", Confidence: "high"},
		Trials:         []agent.Trial{{Spec: agent.TrialSpec{Engine: "vllm"}}},
		Usage:          agent.Usage{Turns: 4},
	}
	r := &fakeRunner{report: report, emit: []agent.Event{
		{Kind: agent.EventPlan, Text: "checking workers"},
		{Kind: agent.EventDone, Text: "done"},
	}}

	require.NoError(t, newTestWorker(store, r).Work(context.Background(), riverJob("camp-1", 3600)))

	assert.True(t, store.completed)
	assert.Equal(t, report, store.completedWith)
	assert.Empty(t, store.failedMsg)
	assert.Len(t, store.events, 2, "campaign progress is recorded as it happens")
}

// River retries and duplicate deliveries must not start a second agent loop
// against a campaign that is already running — that would double-spend both the
// trial budget and the planner tokens.
func TestWorker_DoesNotRunAnUnclaimableCampaign(t *testing.T) {
	store := &fakeStore{campaign: testCampaignRow(), claimable: false}
	r := &fakeRunner{report: &agent.Report{}}

	require.NoError(t, newTestWorker(store, r).Work(context.Background(), riverJob("camp-1", 3600)))

	assert.Equal(t, 1, store.claimCalls)
	assert.False(t, store.completed, "an unclaimed campaign must not be completed")
	assert.Empty(t, store.failedMsg)
}

func TestWorker_RecordsFailureWithoutAskingRiverToRetry(t *testing.T) {
	store := &fakeStore{campaign: testCampaignRow(), claimable: true}
	r := &fakeRunner{err: errors.New("planner turn 1: 401 unauthorized")}

	// nil, not an error: the campaign's outcome lives in its row. Returning an
	// error would have River re-run a campaign whose budget is already spent.
	err := newTestWorker(store, r).Work(context.Background(), riverJob("camp-1", 3600))

	require.NoError(t, err)
	assert.Contains(t, store.failedMsg, "401 unauthorized")
	assert.False(t, store.completed)
}

func TestWorker_CancelledCampaignKeepsItsPartialWork(t *testing.T) {
	store := &fakeStore{campaign: testCampaignRow(), claimable: true}
	partial := &agent.Report{Trials: []agent.Trial{{Spec: agent.TrialSpec{Engine: "vllm"}}}}
	r := &fakeRunner{report: partial, err: ErrCampaignCancelled}

	require.NoError(t, newTestWorker(store, r).Work(context.Background(), riverJob("camp-1", 3600)))

	assert.True(t, store.completed, "a cancelled campaign is finalized, not failed")
	assert.Equal(t, partial, store.completedWith)
	assert.Empty(t, store.failedMsg)
}

func TestWorker_MissingCampaignIsDropped(t *testing.T) {
	store := &fakeStore{getErr: ErrNotFound}
	r := &fakeRunner{report: &agent.Report{}}

	// Retrying a row that no longer exists will never succeed.
	require.NoError(t, newTestWorker(store, r).Work(context.Background(), riverJob("gone", 3600)))
	assert.Zero(t, store.claimCalls)
}

func TestWorker_TimeoutTracksTheCampaignBudget(t *testing.T) {
	w := NewWorker(&fakeStore{}, PlatformDeps{}, nil)

	assert.Equal(t, 30*time.Minute+10*time.Minute, w.Timeout(riverJob("c", 1800)),
		"the River timeout is the wall-clock budget plus slack for a final turn")
	assert.Equal(t, agent.DefaultBudget().MaxDuration+10*time.Minute, w.Timeout(riverJob("c", 0)),
		"an unset budget falls back to the agent default rather than never timing out")
}

func TestWorker_EventRecordingFailureDoesNotAbortTheRun(t *testing.T) {
	store := &failingEventStore{fakeStore: fakeStore{campaign: testCampaignRow(), claimable: true}}
	r := &fakeRunner{report: &agent.Report{}, emit: []agent.Event{{Kind: agent.EventPlan, Text: "x"}}}

	require.NoError(t, newTestWorker(store, r).Work(context.Background(), riverJob("camp-1", 3600)))
	assert.True(t, store.completed, "a campaign that works must not be lost to a logging failure")
}

type failingEventStore struct{ fakeStore }

func (f *failingEventStore) AppendEvent(context.Context, string, agent.Event) error {
	return errors.New("events table unavailable")
}
