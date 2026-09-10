package gateway

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/inferbolthq/inferbolt/internal/agent"
	iauth "github.com/inferbolthq/inferbolt/internal/auth"
	"github.com/inferbolthq/inferbolt/internal/campaigns"
	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/queue"
)

// Ceilings on what one campaign may ask for. The agent enforces its own budget
// internally; these stop a caller declaring a budget that would occupy a worker
// for a week.
const (
	maxCampaignTrials   = 50
	maxCampaignDuration = 24 * time.Hour
	maxEventPageSize    = 500
)

type CreateCampaignRequest struct {
	Goal         string              `json:"goal"`
	Model        string              `json:"model"`
	Engines      []string            `json:"engines"`
	GPUProfile   string              `json:"gpu_profile"`
	Workload     jobs.WorkloadConfig `json:"workload"`
	MaxTrials    int                 `json:"max_trials"`
	MaxDuration  string              `json:"max_duration"`
	PlannerModel string              `json:"planner_model"`
}

// ── POST /v1/campaigns ────────────────────────────────────────────────────────

func (h *Handler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req CreateCampaignRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if req.Goal == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "goal is required"})
		return
	}
	if req.Model == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model is required"})
		return
	}
	if req.GPUProfile == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gpu_profile is required"})
		return
	}
	if len(req.Engines) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one candidate engine is required"})
		return
	}
	if err := jobs.ValidateEngines(req.Engines); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := jobs.ValidateWorkload(req.Workload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	budget, err := campaignBudget(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	plannerModel := req.PlannerModel
	if plannerModel == "" {
		plannerModel = agent.DefaultModel
	}

	tenantID := iauth.MustGetTenantID(r.Context())
	c := campaigns.Campaign{
		ID:           jobs.NewID(),
		TenantID:     tenantID,
		Goal:         req.Goal,
		Model:        req.Model,
		GPUProfile:   req.GPUProfile,
		Engines:      req.Engines,
		Workload:     req.Workload,
		Budget:       budget,
		PlannerModel: plannerModel,
		State:        campaigns.StatePending,
		CreatedAt:    time.Now().UTC(),
	}

	if err := h.campaigns.Create(r.Context(), c); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save campaign"})
		return
	}

	if err := h.queue.EnqueueCampaign(r.Context(), queue.CampaignJobArgs{
		CampaignID:     c.ID,
		TenantID:       tenantID,
		MaxDurationSec: int(budget.MaxDuration.Seconds()),
	}); err != nil {
		// The row exists but nothing will run it; say so rather than reporting
		// a campaign that will sit pending forever.
		_ = h.campaigns.Cancel(r.Context(), c.ID, tenantID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to enqueue campaign"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"campaign_id": c.ID,
		"state":       string(campaigns.StatePending),
	})
}

// campaignBudget applies defaults and rejects anything past the service ceiling.
func campaignBudget(req CreateCampaignRequest) (agent.Budget, error) {
	b := agent.DefaultBudget()

	if req.MaxTrials > 0 {
		b.MaxTrials = req.MaxTrials
	}
	if req.MaxDuration != "" {
		d, err := time.ParseDuration(req.MaxDuration)
		if err != nil {
			return b, apiError("max_duration must be a duration such as \"90m\" or \"2h\"")
		}
		b.MaxDuration = d
	}

	switch {
	case b.MaxTrials > maxCampaignTrials:
		return b, apiError("max_trials must not exceed 50")
	case b.MaxDuration > maxCampaignDuration:
		return b, apiError("max_duration must not exceed 24h")
	}
	if err := b.Validate(); err != nil {
		return b, apiError(err.Error())
	}
	return b, nil
}

// ── GET /v1/campaigns ─────────────────────────────────────────────────────────

func (h *Handler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	tenantID := iauth.MustGetTenantID(r.Context())
	state := r.URL.Query().Get("state")
	limit := queryInt(r, "limit", 20, 100)
	offset := queryInt(r, "offset", 0, -1)

	list, err := h.campaigns.List(r.Context(), tenantID, state, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list campaigns"})
		return
	}
	total, err := h.campaigns.Count(r.Context(), tenantID, state)
	if err != nil {
		total = len(list)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"campaigns": list,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// ── GET /v1/campaigns/{campaignID} ────────────────────────────────────────────

func (h *Handler) GetCampaign(w http.ResponseWriter, r *http.Request) {
	tenantID := iauth.MustGetTenantID(r.Context())
	c, err := h.campaigns.Get(r.Context(), chi.URLParam(r, "campaignID"), tenantID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// ── GET /v1/campaigns/{campaignID}/events ─────────────────────────────────────

// GetCampaignEvents returns steps after ?after_seq, so a client that reconnects
// resumes where it left off instead of replaying the whole campaign.
func (h *Handler) GetCampaignEvents(w http.ResponseWriter, r *http.Request) {
	tenantID := iauth.MustGetTenantID(r.Context())
	campaignID := chi.URLParam(r, "campaignID")

	// Ownership first: never let an event stream confirm that someone else's
	// campaign exists.
	c, err := h.campaigns.Get(r.Context(), campaignID, tenantID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}

	afterSeq := queryInt(r, "after_seq", 0, -1)
	limit := queryInt(r, "limit", 200, maxEventPageSize)

	events, err := h.campaigns.Events(r.Context(), campaignID, afterSeq, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read campaign events"})
		return
	}

	lastSeq := afterSeq
	if n := len(events); n > 0 {
		lastSeq = events[n-1].Seq
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"events":   events,
		"last_seq": lastSeq,
		"state":    string(c.State),
		"done":     campaigns.IsTerminal(c.State),
	})
}

// ── DELETE /v1/campaigns/{campaignID} ─────────────────────────────────────────

func (h *Handler) CancelCampaign(w http.ResponseWriter, r *http.Request) {
	tenantID := iauth.MustGetTenantID(r.Context())

	err := h.campaigns.Cancel(r.Context(), chi.URLParam(r, "campaignID"), tenantID)
	switch {
	case errors.Is(err, campaigns.ErrNotFound):
		// Either it does not exist, is not this tenant's, or has already
		// finished. All three are 404 rather than a distinguishing error.
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no cancellable campaign with that id"})
		return
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to cancel campaign"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}
