package auth

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// corsMaxAge is how long a browser may cache a preflight result. Long enough
// that a dashboard polling every two seconds does not re-preflight constantly.
const corsMaxAge = 10 * time.Minute

// CORSMiddleware allows browser clients from an explicit origin allowlist.
//
// The allowlist is explicit rather than "*" on purpose: these routes take an
// Authorization header, and a wildcard origin cannot be combined with
// credentialed requests — the browser refuses it. Passing no origins disables
// CORS entirely, which is the right default for a deployment whose only clients
// are the CLI and other services.
//
// Preflight is answered here, before authentication, because an OPTIONS
// preflight carries no Authorization header by design; letting it reach
// AuthMiddleware would 401 every cross-origin request before the real one was
// ever sent.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o = strings.TrimSpace(o); o != "" {
			allowed[strings.ToLower(o)] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Any response can vary by Origin once this middleware is in play,
			// including the ones it declines to annotate — otherwise a shared
			// cache can serve one origin's response to another.
			w.Header().Add("Vary", "Origin")

			if origin == "" || !allowed[strings.ToLower(origin)] {
				if r.Method == http.MethodOptions && origin != "" {
					// A preflight from an origin we do not allow: answer it
					// without the headers rather than passing it downstream.
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(int(corsMaxAge.Seconds())))
				w.WriteHeader(http.StatusNoContent)
				return
			}

			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			next.ServeHTTP(w, r)
		})
	}
}

// DefaultDevOrigins are the Vite dev server's addresses. Applied only when the
// gateway runs in development and CORS_ALLOWED_ORIGINS is unset, so that
// `npm run dev` works out of the box without loosening a real deployment.
var DefaultDevOrigins = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}
