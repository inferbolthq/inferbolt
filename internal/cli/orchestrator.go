package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/inferbolthq/inferbolt/internal/workers"
)

// OrchestratorClient talks to the orchestrator's unauthenticated /internal/* routes —
// used only by `inferbolt run` to check worker availability, not the CLI's normal path.
type OrchestratorClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewOrchestratorClient(baseURL string) *OrchestratorClient {
	return &OrchestratorClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *OrchestratorClient) ListWorkers(ctx context.Context) ([]workers.WorkerEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/workers", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reach orchestrator at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("orchestrator returned %d", resp.StatusCode)
	}
	var out []workers.WorkerEntry
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode worker list: %w", err)
	}
	return out, nil
}

func (c *OrchestratorClient) HasIdleWorker(ctx context.Context, gpuProfile string) (bool, error) {
	list, err := c.ListWorkers(ctx)
	if err != nil {
		return false, err
	}
	for _, w := range list {
		if w.Status == workers.StatusIdle && w.GPUType == gpuProfile {
			return true, nil
		}
	}
	return false, nil
}
