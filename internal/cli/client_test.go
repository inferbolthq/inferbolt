package cli_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/inferbolthq/inferbolt/internal/cli"
)

func TestClient_CancelJob_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/v1/jobs/job-123", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"cancelled":true}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := cli.NewClient(srv.URL, "test-key")
	err := c.CancelJob(context.Background(), "job-123")
	require.NoError(t, err)
}

func TestClient_CancelJob_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"job already in terminal state"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := cli.NewClient(srv.URL, "test-key")
	err := c.CancelJob(context.Background(), "job-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job already in terminal state")
}
