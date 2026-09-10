package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	iauth "github.com/inferbolthq/inferbolt/internal/auth"
	"github.com/inferbolthq/inferbolt/internal/campaigns"
	"github.com/inferbolthq/inferbolt/internal/config"
	"github.com/inferbolthq/inferbolt/internal/gateway"
	"github.com/inferbolthq/inferbolt/internal/metrics"
	"github.com/inferbolthq/inferbolt/internal/queue"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	databaseURL := mustEnv("DATABASE_URL")
	jwtSecret := mustEnv("JWT_SECRET")
	orchestratorURL := mustEnv("ORCHESTRATOR_URL")

	if len(jwtSecret) < 32 {
		slog.Error("JWT_SECRET must be at least 32 characters")
		os.Exit(1)
	}

	port := getenv("PORT", "8080")
	env := getenv("ENV", "development")
	otelEndpoint := getenv("OTEL_ENDPOINT", "")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		slog.Error("failed to create db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     200 << 20,
		BufferItems: 64,
	})
	if err != nil {
		slog.Error("failed to create cache", "error", err)
		os.Exit(1)
	}
	defer cache.Close()

	km := iauth.NewKeyManager(jwtSecret, cache)
	mw := metrics.NewMetricsWriter(pool)
	store := config.NewStore(pool)

	queueClient, err := queue.NewQueueClient(ctx, pool)
	if err != nil {
		slog.Error("failed to create queue client", "error", err)
		os.Exit(1)
	}

	if otelEndpoint != "" {
		slog.Info("OTel endpoint configured; set SDK env vars to enable exporter", "endpoint", otelEndpoint)
	}
	tracer := otel.Tracer("inferbolt/gateway")

	h := gateway.NewHandler(gateway.HandlerDeps{
		Jobs:            store,
		Queue:           queueClient,
		Metrics:         mw,
		Campaigns:       campaigns.NewStore(pool),
		Keys:            km,
		Pinger:          pool,
		Pool:            pool,
		OrchestratorURL: orchestratorURL,
	})

	r := chi.NewRouter()
	r.Use(iauth.RequestIDMiddleware())
	r.Use(iauth.LoggerMiddleware())
	r.Use(iauth.OTelMiddleware(tracer))
	r.Use(iauth.TimeoutMiddleware(30 * time.Second))

	r.Get("/health", h.Health)
	r.Get("/dashboard", gateway.Dashboard)

	r.Group(func(r chi.Router) {
		r.Use(iauth.AuthMiddleware(km))
		r.Use(iauth.RateLimitMiddleware(cache))

		r.Group(func(r chi.Router) {
			r.Use(iauth.RequireScope(km, iauth.ScopeJobsWrite))
			r.Post("/v1/jobs", h.CreateJob)
			r.Delete("/v1/jobs/{jobID}", h.CancelJob)
			r.Post("/v1/campaigns", h.CreateCampaign)
			r.Delete("/v1/campaigns/{campaignID}", h.CancelCampaign)
		})

		r.Group(func(r chi.Router) {
			r.Use(iauth.RequireScope(km, iauth.ScopeJobsRead))
			r.Get("/v1/jobs", h.ListJobs)
			r.Get("/v1/jobs/{jobID}", h.GetJob)
			r.Get("/v1/jobs/{jobID}/results", h.GetJobResults)
			r.Post("/v1/route", h.ClassifyWorkload)
			r.Get("/v1/engines", h.ListEngines)
			r.Get("/v1/workers", h.ListWorkers)
			r.Get("/v1/campaigns", h.ListCampaigns)
			r.Get("/v1/campaigns/{campaignID}", h.GetCampaign)
			r.Get("/v1/campaigns/{campaignID}/events", h.GetCampaignEvents)
		})

		r.Group(func(r chi.Router) {
			r.Use(iauth.RequireScope(km, iauth.ScopeMetricsRead))
			r.Get("/v1/metrics", h.GetMetrics)
		})

		r.Group(func(r chi.Router) {
			r.Use(iauth.RequireScope(km, iauth.ScopeAdminAll))
			r.Post("/v1/admin/apikeys", h.CreateAPIKey)
		})
	})

	if env == "development" {
		if err := bootstrapDevKey(ctx, pool, km); err != nil {
			slog.Warn("dev key bootstrap failed", "error", err)
		}
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("gateway listening", "port", port, "env", env)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("shutdown complete")
}

// bootstrapDevKey issues an all-scopes API key for the "dev" tenant on first startup in
// development mode. If DEV_TOKEN_FILE is set, the plaintext is also written there (0600)
// so it survives restarts instead of only ever appearing once in the log.
func bootstrapDevKey(ctx context.Context, pool *pgxpool.Pool, km *iauth.KeyManager) error {
	tokenFile := os.Getenv("DEV_TOKEN_FILE")

	if tokenFile != "" {
		if existing, err := os.ReadFile(tokenFile); err == nil {
			if token := strings.TrimSpace(string(existing)); token != "" && devKeyStillValid(ctx, pool, token) {
				slog.Info("dev API key already bootstrapped", "token_file", tokenFile)
				return nil
			}
		}
	} else {
		var count int
		if err := pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM public.api_keys WHERE tenant_id = 'dev'",
		).Scan(&count); err != nil {
			return fmt.Errorf("check existing dev keys: %w", err)
		}
		if count > 0 {
			return nil
		}
	}

	scopes := []iauth.Scope{
		iauth.ScopeJobsWrite, iauth.ScopeJobsRead,
		iauth.ScopeMetricsRead, iauth.ScopeConfigWrite, iauth.ScopeAdminAll,
	}
	token, err := km.Issue("dev", scopes, 30*24*time.Hour)
	if err != nil {
		return fmt.Errorf("issue dev key: %w", err)
	}

	sum := sha256.Sum256([]byte(token))
	keyHash := hex.EncodeToString(sum[:])
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	_, err = pool.Exec(ctx,
		`INSERT INTO public.api_keys (tenant_id, key_hash, scopes, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		"dev", keyHash, []string{string(iauth.ScopeAdminAll)}, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("persist dev key: %w", err)
	}

	if tokenFile != "" {
		if err := os.MkdirAll(filepath.Dir(tokenFile), 0o700); err != nil {
			slog.Warn("failed to create dev token file directory; token will only be available in logs", "error", err)
		} else if err := os.WriteFile(tokenFile, []byte(token+"\n"), 0o600); err != nil {
			slog.Warn("failed to write dev token file; token will only be available in logs", "error", err)
		}
	}

	slog.Info("dev API key created", "token", token)
	return nil
}

// devKeyStillValid reports whether token still hashes to a live, unexpired, unrevoked
// key for the dev tenant — i.e. whether the on-disk copy is still safe to reuse.
func devKeyStillValid(ctx context.Context, pool *pgxpool.Pool, token string) bool {
	sum := sha256.Sum256([]byte(token))
	keyHash := hex.EncodeToString(sum[:])
	var count int
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.api_keys
		 WHERE tenant_id = 'dev' AND key_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		keyHash,
	).Scan(&count)
	return err == nil && count > 0
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var not set", "var", key)
		os.Exit(1)
	}
	return v
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
