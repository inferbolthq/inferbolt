package gateway

import (
	_ "embed"
	"net/http"
)

//go:embed static/dashboard.html
var dashboardHTML []byte

// Dashboard serves the minimal read-only dashboard — a single static page with no
// build step, calling the existing /v1/jobs, /v1/jobs/{id}/results, and /v1/workers
// endpoints client-side with a user-supplied bearer token.
func Dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(dashboardHTML) //nolint:errcheck
}
