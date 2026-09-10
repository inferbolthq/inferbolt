package gateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	iauth "github.com/inferbolthq/inferbolt/internal/auth"
	"github.com/inferbolthq/inferbolt/internal/gateway"
	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/queue"
)

// ── mock implementations ───────────────────────────────────────────────────────

type mockStore struct {
	job       *jobs.Job
	getErr    error
	saveErr   error
	updateErr error
}

func (m *mockStore) SaveJob(_ context.Context, j jobs.Job) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	cp := j
	m.job = &cp
	return nil
}

func (m *mockStore) GetJob(_ context.Context, jobID, tenantID string) (*jobs.Job, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.job != nil && m.job.ID == jobID && m.job.TenantID == tenantID {
		return m.job, nil
	}
	return nil, errors.New("not found")
}

func (m *mockStore) ListJobs(_ context.Context, _, _ string, _, _ int) ([]jobs.Job, error) {
	return []jobs.Job{}, nil
}

func (m *mockStore) CountJobs(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

func (m *mockStore) UpdateJobState(_ context.Context, _ string, _ jobs.JobState, _ string) error {
	return m.updateErr
}

type mockQueue struct {
	enqueueErr error
	lastArgs   queue.BenchmarkJobArgs
}

func (m *mockQueue) Enqueue(_ context.Context, args queue.BenchmarkJobArgs) error {
	m.lastArgs = args
	return m.enqueueErr
}

type mockMetrics struct {
	lastTenantID string
	results      []jobs.Result
}

func (m *mockMetrics) QueryByJob(_ context.Context, _ string) ([]jobs.Result, error) {
	return nil, nil
}

func (m *mockMetrics) QueryByTenantEngineAndModel(_ context.Context, tenantID, _, _ string, _ time.Time) ([]jobs.Result, error) {
	m.lastTenantID = tenantID
	return m.results, nil
}

type mockPinger struct{ err error }

func (m *mockPinger) Ping(_ context.Context) error { return m.err }

// ── factory helpers ────────────────────────────────────────────────────────────

func newHandler(t *testing.T, store gateway.JobStorer, q gateway.JobQueuer, pinger gateway.DBPinger) *gateway.Handler {
	t.Helper()
	return newHandlerWithMetrics(t, store, q, pinger, &mockMetrics{})
}

func newHandlerWithMetrics(t *testing.T, store gateway.JobStorer, q gateway.JobQueuer, pinger gateway.DBPinger, m gateway.MetricsReader) *gateway.Handler {
	t.Helper()
	c := newCache(t) // defined in middleware_test.go (same package)
	km := iauth.NewKeyManager("test-secret-must-be-32-chars-long!!", c)
	return gateway.NewHandler(store, q, m, km, pinger, nil, "http://localhost:9999")
}

func withTenant(r *http.Request, tenantID string) *http.Request {
	return r.WithContext(iauth.SetTenantID(r.Context(), tenantID))
}

func withChiParam(r *http.Request, key, val string) *http.Request {
	rc := chi.NewRouteContext()
	rc.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

// ── CreateJob ──────────────────────────────────────────────────────────────────

func TestCreateJob_Valid(t *testing.T) {
	h := newHandler(t, &mockStore{}, &mockQueue{}, &mockPinger{})

	body := jsonBody(t, map[string]any{
		"model":       "meta-llama/Llama-3.1-8B",
		"engines":     []string{"vllm"},
		"gpu_profile": "a100-80gb",
		"workload":    map[string]any{"concurrency": 32, "prompt_tokens": 512, "output_tokens": 256, "num_requests": 100},
	})
	r := withTenant(httptest.NewRequest(http.MethodPost, "/v1/jobs", body), "tenant1")
	w := httptest.NewRecorder()
	h.CreateJob(w, r)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["job_id"])
}

func TestCreateJob_AutoRouteWithoutEngines(t *testing.T) {
	// Regression: the handler used to require len(engines) > 0 even when auto_route
	// was set, contradicting the CLI's own validation (--engines optional with --auto-route).
	h := newHandler(t, &mockStore{}, &mockQueue{}, &mockPinger{})

	body := jsonBody(t, map[string]any{
		"model":       "meta-llama/Llama-3.1-8B",
		"gpu_profile": "a100-80gb",
		"auto_route":  true,
		"workload":    map[string]any{"concurrency": 32, "prompt_tokens": 512, "output_tokens": 256, "num_requests": 100},
	})
	r := withTenant(httptest.NewRequest(http.MethodPost, "/v1/jobs", body), "tenant1")
	w := httptest.NewRecorder()
	h.CreateJob(w, r)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["recommended_engine"])
}

func TestCreateJob_MissingModel(t *testing.T) {
	h := newHandler(t, &mockStore{}, &mockQueue{}, &mockPinger{})

	body := jsonBody(t, map[string]any{"engines": []string{"vllm"}, "gpu_profile": "a100-80gb"})
	r := withTenant(httptest.NewRequest(http.MethodPost, "/v1/jobs", body), "tenant1")
	w := httptest.NewRecorder()
	h.CreateJob(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "model")
}

func TestCreateJob_InvalidEngine(t *testing.T) {
	h := newHandler(t, &mockStore{}, &mockQueue{}, &mockPinger{})

	body := jsonBody(t, map[string]any{
		"model":       "some-model",
		"engines":     []string{"unknown-engine"},
		"gpu_profile": "a100-80gb",
	})
	r := withTenant(httptest.NewRequest(http.MethodPost, "/v1/jobs", body), "tenant1")
	w := httptest.NewRecorder()
	h.CreateJob(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── GetJob ─────────────────────────────────────────────────────────────────────

func TestGetJob_NotFound(t *testing.T) {
	store := &mockStore{getErr: errors.New("not found")}
	h := newHandler(t, store, &mockQueue{}, &mockPinger{})

	r := withTenant(httptest.NewRequest(http.MethodGet, "/v1/jobs/missing", nil), "tenant1")
	r = withChiParam(r, "jobID", "missing")
	w := httptest.NewRecorder()
	h.GetJob(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetJob_WrongTenant(t *testing.T) {
	// Store returns job owned by tenant1, but request comes from tenant2
	store := &mockStore{
		job: &jobs.Job{ID: "job-abc", TenantID: "tenant1", State: jobs.StatePending},
	}
	h := newHandler(t, store, &mockQueue{}, &mockPinger{})

	r := httptest.NewRequest(http.MethodGet, "/v1/jobs/job-abc", nil)
	r = withTenant(r, "tenant2")
	r = withChiParam(r, "jobID", "job-abc")
	w := httptest.NewRecorder()
	h.GetJob(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── CancelJob ──────────────────────────────────────────────────────────────────

func TestCancelJob_AlreadyCompleted(t *testing.T) {
	store := &mockStore{
		job: &jobs.Job{ID: "job-done", TenantID: "tenant1", State: jobs.StateCompleted},
	}
	h := newHandler(t, store, &mockQueue{}, &mockPinger{})

	r := httptest.NewRequest(http.MethodDelete, "/v1/jobs/job-done", nil)
	r = withTenant(r, "tenant1")
	r = withChiParam(r, "jobID", "job-done")
	w := httptest.NewRecorder()
	h.CancelJob(w, r)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "terminal")
}

// ── Health ──────────────────────────────────────────────────────────────────────

func TestHealth_DBUp(t *testing.T) {
	h := newHandler(t, &mockStore{}, &mockQueue{}, &mockPinger{err: nil})

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "ok", resp["postgres"])
}

// ── ListWorkers ───────────────────────────────────────────────────────────────

func TestListWorkers_RelaysOrchestratorResponse(t *testing.T) {
	orch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internal/workers", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"w1","url":"http://127.0.0.1:9101","gpu_type":"cpu","status":"idle"}]`)) //nolint:errcheck
	}))
	defer orch.Close()

	c := newCache(t)
	km := iauth.NewKeyManager("test-secret-must-be-32-chars-long!!", c)
	h := gateway.NewHandler(&mockStore{}, &mockQueue{}, &mockMetrics{}, km, &mockPinger{}, nil, orch.URL)

	r := httptest.NewRequest(http.MethodGet, "/v1/workers", nil)
	w := httptest.NewRecorder()
	h.ListWorkers(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"gpu_type":"cpu"`)
}

func TestListWorkers_OrchestratorUnreachable(t *testing.T) {
	c := newCache(t)
	km := iauth.NewKeyManager("test-secret-must-be-32-chars-long!!", c)
	h := gateway.NewHandler(&mockStore{}, &mockQueue{}, &mockMetrics{}, km, &mockPinger{}, nil, "http://127.0.0.1:0")

	r := httptest.NewRequest(http.MethodGet, "/v1/workers", nil)
	w := httptest.NewRecorder()
	h.ListWorkers(w, r)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// ── engine_config / workload validation ────────────────────────────────────────

func validCreateBody(overrides map[string]any) map[string]any {
	body := map[string]any{
		"model":       "meta-llama/Llama-3.1-8B",
		"engines":     []string{"vllm"},
		"gpu_profile": "a100-80gb",
		"workload":    map[string]any{"concurrency": 32, "prompt_tokens": 512, "output_tokens": 256, "num_requests": 100},
	}
	for k, v := range overrides {
		body[k] = v
	}
	return body
}

func TestCreateJob_ValidatesEngineConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]any
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "omitted entirely is valid — worker applies its own defaults",
			config:     nil,
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "zero values mean unset, not invalid",
			config:     map[string]any{"tensor_parallel": 0, "max_batch_size": 0, "gpu_memory_utilization": 0},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "fully specified",
			config:     map[string]any{"quantization": "fp8", "tensor_parallel": 2, "max_batch_size": 256, "max_model_len": 8192, "gpu_memory_utilization": 0.9},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "unknown quantization is rejected, not passed through to the engine CLI",
			config:     map[string]any{"quantization": "--rm -rf"},
			wantStatus: http.StatusBadRequest,
			wantMsg:    "unknown quantization",
		},
		{
			name:       "tensor_parallel above the supported range",
			config:     map[string]any{"tensor_parallel": 16},
			wantStatus: http.StatusBadRequest,
			wantMsg:    "tensor_parallel",
		},
		{
			name:       "negative tensor_parallel",
			config:     map[string]any{"tensor_parallel": -1},
			wantStatus: http.StatusBadRequest,
			wantMsg:    "tensor_parallel",
		},
		{
			name:       "max_batch_size above the supported range",
			config:     map[string]any{"max_batch_size": 100000},
			wantStatus: http.StatusBadRequest,
			wantMsg:    "max_batch_size",
		},
		{
			name:       "gpu_memory_utilization above 1",
			config:     map[string]any{"gpu_memory_utilization": 1.5},
			wantStatus: http.StatusBadRequest,
			wantMsg:    "gpu_memory_utilization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandler(t, &mockStore{}, &mockQueue{}, &mockPinger{})
			overrides := map[string]any{}
			if tt.config != nil {
				overrides["engine_config"] = tt.config
			}
			body := jsonBody(t, validCreateBody(overrides))
			r := withTenant(httptest.NewRequest(http.MethodPost, "/v1/jobs", body), "tenant1")
			w := httptest.NewRecorder()
			h.CreateJob(w, r)

			assert.Equal(t, tt.wantStatus, w.Code, w.Body.String())
			if tt.wantMsg != "" {
				assert.Contains(t, w.Body.String(), tt.wantMsg)
			}
		})
	}
}

func TestCreateJob_ValidatesWorkload(t *testing.T) {
	tests := []struct {
		name     string
		workload map[string]any
		wantMsg  string
	}{
		{
			// Semaphore(0) on the worker means the benchmark never starts and the
			// job hangs until the orchestrator's 45-minute poll timeout.
			name:     "zero concurrency is rejected rather than dispatched",
			workload: map[string]any{"concurrency": 0, "prompt_tokens": 512, "output_tokens": 256, "num_requests": 100},
			wantMsg:  "concurrency",
		},
		{
			name:     "zero num_requests",
			workload: map[string]any{"concurrency": 32, "prompt_tokens": 512, "output_tokens": 256, "num_requests": 0},
			wantMsg:  "num_requests",
		},
		{
			name:     "negative prompt_tokens",
			workload: map[string]any{"concurrency": 32, "prompt_tokens": -1, "output_tokens": 256, "num_requests": 100},
			wantMsg:  "prompt_tokens",
		},
		{
			name:     "output_tokens beyond the cap",
			workload: map[string]any{"concurrency": 32, "prompt_tokens": 512, "output_tokens": 9_000_000, "num_requests": 100},
			wantMsg:  "output_tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandler(t, &mockStore{}, &mockQueue{}, &mockPinger{})
			body := jsonBody(t, validCreateBody(map[string]any{"workload": tt.workload}))
			r := withTenant(httptest.NewRequest(http.MethodPost, "/v1/jobs", body), "tenant1")
			w := httptest.NewRecorder()
			h.CreateJob(w, r)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantMsg)
		})
	}
}

// The whole point of the engine_config path: what the caller asked for has to
// survive persistence and reach the worker dispatch payload unchanged.
func TestCreateJob_EngineConfigReachesStoreAndQueue(t *testing.T) {
	store := &mockStore{}
	q := &mockQueue{}
	h := newHandler(t, store, q, &mockPinger{})

	body := jsonBody(t, validCreateBody(map[string]any{
		"engine_config": map[string]any{"quantization": "int4", "tensor_parallel": 4, "max_batch_size": 512},
	}))
	r := withTenant(httptest.NewRequest(http.MethodPost, "/v1/jobs", body), "tenant1")
	w := httptest.NewRecorder()
	h.CreateJob(w, r)
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

	require.NotNil(t, store.job)
	assert.Equal(t, "int4", store.job.EngineConfig.Quantization)
	assert.Equal(t, 4, store.job.EngineConfig.TensorParallel)
	assert.Equal(t, 512, store.job.EngineConfig.MaxBatchSize)

	assert.Equal(t, "int4", q.lastArgs.EngineConfig.Quantization)
	assert.Equal(t, 4, q.lastArgs.EngineConfig.TensorParallel)
	assert.Equal(t, 512, q.lastArgs.EngineConfig.MaxBatchSize)
}

// Unset fields must not serialize as explicit zeros — the worker treats 0 as a
// real value for tensor_parallel and would start an engine with nonsense flags.
func TestBenchmarkJobArgs_OmitsUnsetEngineConfig(t *testing.T) {
	b, err := json.Marshal(queue.BenchmarkJobArgs{
		JobID:        "job-1",
		EngineConfig: queue.EngineConfig{Quantization: "fp8"},
	})
	require.NoError(t, err)
	assert.Contains(t, string(b), `"quantization":"fp8"`)
	assert.NotContains(t, string(b), "tensor_parallel")
	assert.NotContains(t, string(b), "max_batch_size")
	assert.NotContains(t, string(b), "gpu_memory_utilization")
}

// ── tenant isolation ──────────────────────────────────────────────────────────

// metrics.bench_results carries no tenant_id, so GET /v1/metrics used to return
// every tenant's results for a given engine and model. Model names are public,
// which made another tenant's throughput, cost and engine configuration
// readable by anyone who could guess one.
func TestGetMetrics_ScopesToTheAuthenticatedTenant(t *testing.T) {
	m := &mockMetrics{}
	h := newHandlerWithMetrics(t, &mockStore{}, &mockQueue{}, &mockPinger{}, m)

	r := withTenant(httptest.NewRequest(http.MethodGet,
		"/v1/metrics?engine=vllm&model=meta-llama/Llama-3.1-8B", nil), "tenant-a")
	w := httptest.NewRecorder()
	h.GetMetrics(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "tenant-a", m.lastTenantID,
		"the tenant must come from the verified token, never from the request")
}
