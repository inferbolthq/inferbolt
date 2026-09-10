package gateway

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	iauth "github.com/inferbolthq/inferbolt/internal/auth"
	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/queue"
	"github.com/inferbolthq/inferbolt/internal/router"
)

// Handler holds all route handler dependencies.
type Handler struct {
	store           JobStorer
	queue           JobQueuer
	metrics         MetricsReader
	km              *iauth.KeyManager
	pinger          DBPinger
	pool            *pgxpool.Pool // used only for API key inserts
	classifier      func(router.ClassificationInput) router.ClassificationResult
	orchestratorURL string
	httpClient      *http.Client
}

// NewHandler constructs a Handler with all required dependencies.
func NewHandler(
	store JobStorer,
	queue JobQueuer,
	metrics MetricsReader,
	km *iauth.KeyManager,
	pinger DBPinger,
	pool *pgxpool.Pool,
	orchestratorURL string,
) *Handler {
	return &Handler{
		store:           store,
		queue:           queue,
		metrics:         metrics,
		km:              km,
		pinger:          pinger,
		pool:            pool,
		classifier:      router.Classify,
		orchestratorURL: orchestratorURL,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
	}
}

func readJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return errBadBody
	}
	return json.NewDecoder(r.Body).Decode(v)
}

type apiError string

func (e apiError) Error() string { return string(e) }

const errBadBody apiError = "empty or malformed request body"

var validEngines = map[string]bool{
	"vllm": true, "sglang": true, "tensorrt": true,
	"llamacpp": true, "ollama": true, "mock": true,
}

// Quantization values are forwarded verbatim as engine CLI arguments
// (vllm --quantization, sglang --quantization), so the set is closed.
var validQuantizations = map[string]bool{
	"fp8": true, "int8": true, "int4": true, "gptq": true, "awq": true,
}

// validateWorkload bounds the shape of a benchmark run. These are rejections,
// not clamps: a caller that asks for something unrunnable should hear about it
// rather than silently get a different benchmark than it requested. Zero
// concurrency is the sharpest edge — it wedges the worker on an
// asyncio.Semaphore(0) until the orchestrator's 45-minute poll timeout fires.
func validateWorkload(w jobs.WorkloadConfig) error {
	switch {
	case w.Concurrency < 1 || w.Concurrency > 1024:
		return apiError("workload.concurrency must be between 1 and 1024")
	case w.PromptTokens < 1 || w.PromptTokens > 1_000_000:
		return apiError("workload.prompt_tokens must be between 1 and 1000000")
	case w.OutputTokens < 1 || w.OutputTokens > 1_000_000:
		return apiError("workload.output_tokens must be between 1 and 1000000")
	case w.NumRequests < 1 || w.NumRequests > 100_000:
		return apiError("workload.num_requests must be between 1 and 100000")
	}
	return nil
}

// validateEngineConfig bounds the engine tuning knobs. A zero value means
// "unset" and is left to the worker's own default, so only non-zero fields
// are range-checked.
func validateEngineConfig(c jobs.EngineConfig) error {
	switch {
	case c.Quantization != "" && !validQuantizations[c.Quantization]:
		return apiError("unknown quantization: " + c.Quantization)
	case c.TensorParallel < 0 || c.TensorParallel > 8:
		return apiError("engine_config.tensor_parallel must be between 1 and 8 (0 = engine default)")
	case c.MaxBatchSize < 0 || c.MaxBatchSize > 4096:
		return apiError("engine_config.max_batch_size must be between 1 and 4096 (0 = engine default)")
	case c.MaxModelLen < 0 || c.MaxModelLen > 1_048_576:
		return apiError("engine_config.max_model_len must be between 1 and 1048576 (0 = engine default)")
	case c.GPUMemoryUtilization < 0 || c.GPUMemoryUtilization > 1:
		return apiError("engine_config.gpu_memory_utilization must be between 0 and 1 (0 = engine default)")
	}
	return nil
}

// ── POST /v1/jobs ─────────────────────────────────────────────────────────────

type CreateJobRequest struct {
	Model        string              `json:"model"`
	Engines      []string            `json:"engines"`
	Workload     jobs.WorkloadConfig `json:"workload"`
	EngineConfig jobs.EngineConfig   `json:"engine_config"`
	GPUProfile   string              `json:"gpu_profile"`
	AutoRoute    bool                `json:"auto_route"`
}

func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if req.Model == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model is required"})
		return
	}
	if !req.AutoRoute && len(req.Engines) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one engine is required (or set auto_route)"})
		return
	}
	for _, e := range req.Engines {
		if !validEngines[e] {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "unknown engine: " + e,
			})
			return
		}
	}
	if req.GPUProfile == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "gpu_profile is required"})
		return
	}
	if err := validateWorkload(req.Workload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := validateEngineConfig(req.EngineConfig); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var recommendedEngine string
	if req.AutoRoute {
		result := h.classifier(router.ClassificationInput{
			PromptTokens: req.Workload.PromptTokens,
			OutputTokens: req.Workload.OutputTokens,
			Concurrency:  req.Workload.Concurrency,
		})
		recommendedEngine = result.RecommendedEngine
		if len(req.Engines) == 0 {
			req.Engines = []string{recommendedEngine}
		} else {
			req.Engines[0] = recommendedEngine
		}
	}

	tenantID := iauth.MustGetTenantID(r.Context())
	now := time.Now().UTC()
	job := jobs.Job{
		ID:             newUUID(),
		TenantID:       tenantID,
		Model:          req.Model,
		Engines:        req.Engines,
		WorkloadConfig: req.Workload,
		EngineConfig:   req.EngineConfig,
		GPUProfile:     req.GPUProfile,
		State:          jobs.StatePending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.store.SaveJob(r.Context(), job); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save job"})
		return
	}

	err := h.queue.Enqueue(r.Context(), queue.BenchmarkJobArgs{
		JobID:        job.ID,
		Model:        job.Model,
		Engines:      job.Engines,
		Workload:     queue.WorkloadConfig(job.WorkloadConfig),
		EngineConfig: queue.EngineConfig(job.EngineConfig),
		GPUProfile:   job.GPUProfile,
		TenantID:     tenantID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to enqueue job"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":             job.ID,
		"state":              string(jobs.StatePending),
		"auto_routed":        req.AutoRoute,
		"recommended_engine": recommendedEngine,
	})
}

// ── GET /v1/jobs/{jobID} ──────────────────────────────────────────────────────

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	tenantID := iauth.MustGetTenantID(r.Context())

	job, err := h.store.GetJob(r.Context(), jobID, tenantID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// ── GET /v1/jobs/{jobID}/results ──────────────────────────────────────────────

func (h *Handler) GetJobResults(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	tenantID := iauth.MustGetTenantID(r.Context())

	// Verify ownership
	if _, err := h.store.GetJob(r.Context(), jobID, tenantID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}

	results, err := h.metrics.QueryByJob(r.Context(), jobID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query results"})
		return
	}
	if results == nil {
		results = []jobs.Result{}
	}
	writeJSON(w, http.StatusOK, results)
}

// ── GET /v1/jobs ──────────────────────────────────────────────────────────────

func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	tenantID := iauth.MustGetTenantID(r.Context())

	state := r.URL.Query().Get("state")
	limit := queryInt(r, "limit", 20, 100)
	offset := queryInt(r, "offset", 0, -1)

	jobList, err := h.store.ListJobs(r.Context(), tenantID, state, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list jobs"})
		return
	}
	if jobList == nil {
		jobList = []jobs.Job{}
	}

	total, err := h.store.CountJobs(r.Context(), tenantID, state)
	if err != nil {
		total = len(jobList)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"jobs":   jobList,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ── DELETE /v1/jobs/{jobID} ───────────────────────────────────────────────────

func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	tenantID := iauth.MustGetTenantID(r.Context())

	job, err := h.store.GetJob(r.Context(), jobID, tenantID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	if jobs.IsTerminal(job.State) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "job already in terminal state"})
		return
	}

	if err := h.store.UpdateJobState(r.Context(), jobID, jobs.StateCancelled, ""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to cancel job"})
		return
	}

	// Notify orchestrator (best-effort)
	go func() {
		req, _ := http.NewRequest(http.MethodPost,
			h.orchestratorURL+"/internal/jobs/"+jobID+"/cancel", nil)
		if req != nil {
			h.httpClient.Do(req) //nolint:errcheck
		}
	}()

	writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}

// ── GET /v1/metrics ───────────────────────────────────────────────────────────

func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	engine := r.URL.Query().Get("engine")
	model := r.URL.Query().Get("model")
	sinceStr := r.URL.Query().Get("since")

	var since time.Time
	if sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid since format (use RFC3339)"})
			return
		}
	} else {
		since = time.Now().Add(-24 * time.Hour)
	}

	tenantID := iauth.MustGetTenantID(r.Context())
	results, err := h.metrics.QueryByTenantEngineAndModel(r.Context(), tenantID, engine, model, since)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query metrics"})
		return
	}
	if results == nil {
		results = []jobs.Result{}
	}
	writeJSON(w, http.StatusOK, results)
}

// ── POST /v1/route ────────────────────────────────────────────────────────────

func (h *Handler) ClassifyWorkload(w http.ResponseWriter, r *http.Request) {
	var input router.ClassificationInput
	if err := readJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.classifier(input))
}

// ── GET /v1/engines ───────────────────────────────────────────────────────────

func (h *Handler) ListEngines(w http.ResponseWriter, r *http.Request) {
	engines := []map[string]string{
		{"name": "vllm", "description": "Broad model support, continuous batching, PagedAttention"},
		{"name": "sglang", "description": "Agent workloads, structured output, RadixAttention"},
		{"name": "tensorrt", "description": "Maximum throughput on NVIDIA hardware"},
		{"name": "llamacpp", "description": "CPU inference, edge deployment, GGUF quantization"},
		{"name": "ollama", "description": "Local development, easy model management"},
		{"name": "mock", "description": "No-GPU smoke test, no real model server"},
	}
	writeJSON(w, http.StatusOK, engines)
}

// ── GET /v1/workers ───────────────────────────────────────────────────────────

// ListWorkers relays the orchestrator's worker registry so browser/CLI clients only
// ever need to talk to the gateway, never the orchestrator's unauthenticated internal port.
func (h *Handler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.orchestratorURL+"/internal/workers", nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to build orchestrator request"})
		return
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "orchestrator unreachable"})
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// ── POST /v1/admin/apikeys ────────────────────────────────────────────────────

type CreateAPIKeyRequest struct {
	TenantID   string   `json:"tenant_id"`
	Scopes     []string `json:"scopes"`
	ExpiryDays int      `json:"expiry_days"`
}

func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.TenantID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant_id is required"})
		return
	}
	if len(req.Scopes) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scopes must not be empty"})
		return
	}
	if req.ExpiryDays < 1 || req.ExpiryDays > 365 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expiry_days must be between 1 and 365"})
		return
	}

	scopes := make([]iauth.Scope, len(req.Scopes))
	for i, s := range req.Scopes {
		scopes[i] = iauth.Scope(s)
	}

	expiry := time.Duration(req.ExpiryDays) * 24 * time.Hour
	token, err := h.km.Issue(req.TenantID, scopes, expiry)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to issue token"})
		return
	}

	// Store SHA256 hash, never the token itself
	sum := sha256.Sum256([]byte(token))
	keyHash := hex.EncodeToString(sum[:])
	expiresAt := time.Now().Add(expiry)

	if h.pool != nil {
		_, err = h.pool.Exec(r.Context(),
			`INSERT INTO public.api_keys (tenant_id, key_hash, scopes, expires_at, created_at)
			 VALUES ($1, $2, $3, $4, NOW())`,
			req.TenantID, keyHash, req.Scopes, expiresAt,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist api key"})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

// ── GET /health ───────────────────────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"status":  "ok",
		"version": "0.1.0",
	}
	if err := h.pinger.Ping(r.Context()); err != nil {
		resp["postgres"] = "error"
	} else {
		resp["postgres"] = "ok"
	}
	writeJSON(w, http.StatusOK, resp)
}

// ── internal helpers ──────────────────────────────────────────────────────────

func queryInt(r *http.Request, key string, defaultVal, maxVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultVal
	}
	if maxVal > 0 && n > maxVal {
		return maxVal
	}
	return n
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = cryptorand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:])
}
