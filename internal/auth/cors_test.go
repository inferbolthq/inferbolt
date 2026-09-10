package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	iauth "github.com/inferbolthq/inferbolt/internal/auth"
)

func corsHandler(origins []string) http.Handler {
	reached := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("downstream")) //nolint:errcheck
	})
	return iauth.CORSMiddleware(origins)(reached)
}

func TestCORS_AllowsAListedOrigin(t *testing.T) {
	h := corsHandler([]string{"http://localhost:5173"})

	r := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, w.Header().Values("Vary"), "Origin")
}

// A preflight carries no Authorization header, so it has to be answered before
// the auth middleware sees it — otherwise every cross-origin request 401s
// before the real request is ever sent.
func TestCORS_AnswersPreflightWithoutReachingDownstream(t *testing.T) {
	h := corsHandler([]string{"http://localhost:5173"})

	r := httptest.NewRequest(http.MethodOptions, "/v1/jobs", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", "authorization")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.NotContains(t, w.Body.String(), "downstream")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "DELETE")
	assert.NotEmpty(t, w.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_DoesNotAnnotateAnUnlistedOrigin(t *testing.T) {
	h := corsHandler([]string{"http://localhost:5173"})

	r := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	r.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	// The request still runs — CORS is enforced by the browser, not the server —
	// but without the header the browser will refuse to hand over the response.
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_RejectsAPreflightFromAnUnlistedOrigin(t *testing.T) {
	h := corsHandler([]string{"http://localhost:5173"})

	r := httptest.NewRequest(http.MethodOptions, "/v1/jobs", nil)
	r.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

// A deployment whose only clients are the CLI and other services should not
// grow a CORS surface just because the middleware is installed.
func TestCORS_DisabledWhenNoOriginsConfigured(t *testing.T) {
	h := corsHandler(nil)

	r := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_IgnoresSameOriginRequests(t *testing.T) {
	h := corsHandler([]string{"http://localhost:5173"})

	r := httptest.NewRequest(http.MethodGet, "/health", nil) // no Origin header
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "downstream", w.Body.String())
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_OriginMatchIsCaseInsensitive(t *testing.T) {
	h := corsHandler([]string{"http://LocalHost:5173"})

	r := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
}
