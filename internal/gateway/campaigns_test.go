package gateway_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/inferbolthq/inferbolt/internal/agent"
	"github.com/inferbolthq/inferbolt/internal/campaigns"
	"github.com/inferbolthq/inferbolt/internal/gateway"
)

func validCampaignBody(overrides map[string]any) map[string]any {
	body := map[string]any{
		"goal":        "cheapest engine under 200ms p99 TTFT",
		"model":       "meta-llama/Llama-3.1-8B",
		"engines":     []string{"vllm", "sglang"},
		"gpu_profile": "a100-80gb",
		"workload":    map[string]any{"concurrency": 32, "prompt_tokens": 512, "output_tokens": 256, "num_requests": 200},
	}
	for k, v := range overrides {
		body[k] = v
	}
	return body
}

func postCampaign(t *testing.T, h *gateway.Handler, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	r := withTenant(httptest.NewRequest(http.MethodPost, "/v1/campaigns", jsonBody(t, body)), "tenant-a")
	w := httptest.NewRecorder()
	h.CreateCampaign(w, r)
	return w
}

func TestCreateCampaign_PersistsAndEnqueues(t *testing.T) {
	store := &mockCampaigns{}
	q := &mockQueue{}
	h := newHandlerWith(t, gateway.HandlerDeps{Campaigns: store, Queue: q})

	w := postCampaign(t, h, validCampaignBody(nil))
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["campaign_id"])
	assert.Equal(t, "pending", resp["state"])

	require.Len(t, store.created, 1)
	c := store.created[0]
	assert.Equal(t, "tenant-a", c.TenantID, "the tenant comes from the token, never the body")
	assert.Equal(t, agent.DefaultModel, c.PlannerModel)
	assert.Equal(t, agent.DefaultBudget().MaxTrials, c.Budget.MaxTrials)
	assert.Equal(t, campaigns.StatePending, c.State)

	require.Equal(t, 1, q.campaignsQueued)
	assert.Equal(t, c.ID, q.lastCampaign.CampaignID)
	assert.Equal(t, "tenant-a", q.lastCampaign.TenantID)
	assert.Equal(t, int(c.Budget.MaxDuration.Seconds()), q.lastCampaign.MaxDurationSec)
}

func TestCreateCampaign_Validation(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]any
		wantMsg   string
	}{
		{"missing goal", map[string]any{"goal": ""}, "goal"},
		{"missing model", map[string]any{"model": ""}, "model"},
		{"missing gpu profile", map[string]any{"gpu_profile": ""}, "gpu_profile"},
		{"no engines", map[string]any{"engines": []string{}}, "engine"},
		{"unknown engine", map[string]any{"engines": []string{"not-an-engine"}}, "unknown engine"},
		{"bad workload", map[string]any{"workload": map[string]any{"concurrency": 0, "prompt_tokens": 512, "output_tokens": 256, "num_requests": 10}}, "concurrency"},
		{"trial budget beyond the ceiling", map[string]any{"max_trials": 5000}, "max_trials"},
		{"duration beyond the ceiling", map[string]any{"max_duration": "72h"}, "max_duration"},
		{"unparseable duration", map[string]any{"max_duration": "soon"}, "max_duration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockCampaigns{}
			q := &mockQueue{}
			h := newHandlerWith(t, gateway.HandlerDeps{Campaigns: store, Queue: q})

			w := postCampaign(t, h, validCampaignBody(tt.overrides))

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantMsg)
			assert.Empty(t, store.created, "a rejected campaign must not be persisted")
			assert.Zero(t, q.campaignsQueued, "a rejected campaign must not be queued")
		})
	}
}

func TestCreateCampaign_HonoursExplicitBudget(t *testing.T) {
	store := &mockCampaigns{}
	h := newHandlerWith(t, gateway.HandlerDeps{Campaigns: store})

	w := postCampaign(t, h, validCampaignBody(map[string]any{
		"max_trials": 3, "max_duration": "45m", "planner_model": "claude-sonnet-5",
	}))
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

	require.Len(t, store.created, 1)
	assert.Equal(t, 3, store.created[0].Budget.MaxTrials)
	assert.Equal(t, "45m0s", store.created[0].Budget.MaxDuration.String())
	assert.Equal(t, "claude-sonnet-5", store.created[0].PlannerModel)
}

// A campaign row that nothing will ever run is worse than a failed request:
// it sits pending forever with no explanation.
func TestCreateCampaign_CancelsTheRowIfEnqueueFails(t *testing.T) {
	store := &mockCampaigns{}
	q := &mockQueue{campaignErr: assertAnError{}}
	h := newHandlerWith(t, gateway.HandlerDeps{Campaigns: store, Queue: q})

	w := postCampaign(t, h, validCampaignBody(nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	require.Len(t, store.created, 1)
	assert.Equal(t, []string{store.created[0].ID}, store.cancelled)
}

type assertAnError struct{}

func (assertAnError) Error() string { return "queue unavailable" }

func TestGetCampaign_NotFoundForAnotherTenant(t *testing.T) {
	store := &mockCampaigns{stored: &campaigns.Campaign{ID: "c1", TenantID: "tenant-a"}}
	h := newHandlerWith(t, gateway.HandlerDeps{Campaigns: store})

	r := withChiParam(
		withTenant(httptest.NewRequest(http.MethodGet, "/v1/campaigns/c1", nil), "tenant-b"),
		"campaignID", "c1")
	w := httptest.NewRecorder()
	h.GetCampaign(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"another tenant's campaign must be indistinguishable from a missing one")
}

func TestGetCampaignEvents_ResumesFromCursor(t *testing.T) {
	store := &mockCampaigns{
		stored: &campaigns.Campaign{ID: "c1", TenantID: "tenant-a", State: campaigns.StateRunning},
		events: []campaigns.Event{
			{Seq: 1, Kind: "plan", Text: "checking workers"},
			{Seq: 2, Kind: "trial", Text: "probing fp8"},
			{Seq: 3, Kind: "result", Text: ""},
		},
	}
	h := newHandlerWith(t, gateway.HandlerDeps{Campaigns: store})

	r := withChiParam(
		withTenant(httptest.NewRequest(http.MethodGet, "/v1/campaigns/c1/events?after_seq=1", nil), "tenant-a"),
		"campaignID", "c1")
	w := httptest.NewRecorder()
	h.GetCampaignEvents(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Events  []campaigns.Event `json:"events"`
		LastSeq int               `json:"last_seq"`
		State   string            `json:"state"`
		Done    bool              `json:"done"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(t, 1, store.lastEventsAfterSeq)
	require.Len(t, resp.Events, 2, "only steps after the cursor")
	assert.Equal(t, 3, resp.LastSeq)
	assert.Equal(t, "running", resp.State)
	assert.False(t, resp.Done, "a running campaign must not tell a poller to stop")
}

func TestGetCampaignEvents_ChecksOwnershipBeforeReading(t *testing.T) {
	store := &mockCampaigns{
		stored: &campaigns.Campaign{ID: "c1", TenantID: "tenant-a"},
		events: []campaigns.Event{{Seq: 1, Kind: "plan", Text: "secret"}},
	}
	h := newHandlerWith(t, gateway.HandlerDeps{Campaigns: store})

	r := withChiParam(
		withTenant(httptest.NewRequest(http.MethodGet, "/v1/campaigns/c1/events", nil), "tenant-b"),
		"campaignID", "c1")
	w := httptest.NewRecorder()
	h.GetCampaignEvents(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), "secret")
}

func TestGetCampaignEvents_SignalsDoneOnTerminalState(t *testing.T) {
	store := &mockCampaigns{
		stored: &campaigns.Campaign{ID: "c1", TenantID: "tenant-a", State: campaigns.StateCompleted},
	}
	h := newHandlerWith(t, gateway.HandlerDeps{Campaigns: store})

	r := withChiParam(
		withTenant(httptest.NewRequest(http.MethodGet, "/v1/campaigns/c1/events", nil), "tenant-a"),
		"campaignID", "c1")
	w := httptest.NewRecorder()
	h.GetCampaignEvents(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"done":true`)
}

func TestCancelCampaign_NotFoundIsNotCancellable(t *testing.T) {
	store := &mockCampaigns{cancelErr: campaigns.ErrNotFound}
	h := newHandlerWith(t, gateway.HandlerDeps{Campaigns: store})

	r := withChiParam(
		withTenant(httptest.NewRequest(http.MethodDelete, "/v1/campaigns/c1", nil), "tenant-a"),
		"campaignID", "c1")
	w := httptest.NewRecorder()
	h.CancelCampaign(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
