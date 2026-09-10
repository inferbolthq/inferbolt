package gateway_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/inferbolthq/inferbolt/internal/gateway"
)

func TestDashboard_ServesHTML(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	gateway.Dashboard(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "InferBolt Dashboard")
	assert.Contains(t, w.Body.String(), "/v1/workers")
}
