package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/inferbolthq/inferbolt/internal/cli"
	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/router"
)

// Set by GoReleaser via ldflags.
var version = "dev"

// ── shared state ──────────────────────────────────────────────────────────────

var (
	cfg       *cli.Config
	apiClient *cli.Client

	serverFlag string
	apiKeyFlag string
	outputFlag string
)

// ── root command ──────────────────────────────────────────────────────────────

var rootCmd = &cobra.Command{
	Use:   "inferbolt",
	Short: "InferBolt — open-source LLM inference optimization toolkit",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// configure and version work without a loaded config; run and agent do
		// their own bootstrap-aware config resolution (they can auto-provision a
		// local dev credential, so a missing API key isn't necessarily fatal).
		name := cmd.Name()
		if name == "configure" || name == "version" || name == "run" || name == "agent" {
			return nil
		}
		var err error
		cfg, err = cli.Load()
		if err != nil {
			return err
		}
		if serverFlag != "" {
			cfg.ServerURL = serverFlag
		}
		if apiKeyFlag != "" {
			cfg.APIKey = apiKeyFlag
		}
		if outputFlag != "" {
			cfg.OutputFmt = outputFlag
		}
		apiClient = cfg.NewClient()
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverFlag, "server", "", "InferBolt server URL")
	rootCmd.PersistentFlags().StringVar(&apiKeyFlag, "api-key", "", "InferBolt API key")
	rootCmd.PersistentFlags().StringVar(&outputFlag, "output", "table", "Output format: table or json")

	rootCmd.AddCommand(newBenchmarkCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newJobsCmd())
	rootCmd.AddCommand(newMetricsCmd())
	rootCmd.AddCommand(newRouteCmd())
	rootCmd.AddCommand(newConfigureCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newAdminCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ── benchmark ─────────────────────────────────────────────────────────────────

func newBenchmarkCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "benchmark", Short: "Run and compare inference benchmarks"}
	cmd.AddCommand(newBenchmarkRunCmd())
	cmd.AddCommand(newBenchmarkCompareCmd())
	return cmd
}

func newBenchmarkRunCmd() *cobra.Command {
	var (
		model        string
		enginesFlag  string
		gpu          string
		concurrency  int
		promptTokens int
		outputTokens int
		requests     int
		autoRoute    bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a benchmark job",
		RunE: func(cmd *cobra.Command, args []string) error {
			if model == "" {
				return fmt.Errorf("--model is required")
			}
			if !autoRoute && enginesFlag == "" {
				return fmt.Errorf("--engines is required unless --auto-route is set")
			}
			if gpu == "" {
				return fmt.Errorf("--gpu is required")
			}

			engines := splitCSV(enginesFlag)

			req := cli.CreateJobRequest{
				Model:   model,
				Engines: engines,
				Workload: jobs.WorkloadConfig{
					Concurrency:  concurrency,
					PromptTokens: promptTokens,
					OutputTokens: outputTokens,
					NumRequests:  requests,
				},
				GPUProfile: gpu,
				AutoRoute:  autoRoute,
			}

			jobResp, err := apiClient.CreateJob(cmd.Context(), req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			engineLabel := strings.Join(engines, ",")
			if jobResp.RecommendedEngine != "" {
				engineLabel = jobResp.RecommendedEngine
			}

			finalJob, results, err := pollAndReport(cmd.Context(), jobResp.JobID,
				fmt.Sprintf("Benchmarking %s on %s", model, engineLabel))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"job_id":  jobResp.JobID,
					"model":   model,
					"state":   string(finalJob.State),
					"engines": engines,
					"results": results,
				})
			}

			printResultsTable(results)
			return nil
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "Model to benchmark (required)")
	cmd.Flags().StringVar(&enginesFlag, "engines", "", "Comma-separated engines (required unless --auto-route)")
	cmd.Flags().StringVar(&gpu, "gpu", "", "GPU profile, e.g. a100-80gb (required)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 32, "Number of concurrent requests")
	cmd.Flags().IntVar(&promptTokens, "prompt-tokens", 512, "Prompt length in tokens")
	cmd.Flags().IntVar(&outputTokens, "output-tokens", 256, "Max output tokens")
	cmd.Flags().IntVar(&requests, "requests", 200, "Total number of requests")
	cmd.Flags().BoolVar(&autoRoute, "auto-route", false, "Auto-select best engine via classifier")
	return cmd
}

func newBenchmarkCompareCmd() *cobra.Command {
	var (
		model        string
		enginesFlag  string
		gpu          string
		concurrency  int
		promptTokens int
		outputTokens int
		requests     int
	)

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare multiple engines in a single benchmark run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if model == "" {
				return fmt.Errorf("--model is required")
			}
			engines := splitCSV(enginesFlag)
			if len(engines) < 2 {
				return fmt.Errorf("--engines requires at least 2 comma-separated engines for comparison")
			}
			if gpu == "" {
				return fmt.Errorf("--gpu is required")
			}

			req := cli.CreateJobRequest{
				Model:   model,
				Engines: engines,
				Workload: jobs.WorkloadConfig{
					Concurrency:  concurrency,
					PromptTokens: promptTokens,
					OutputTokens: outputTokens,
					NumRequests:  requests,
				},
				GPUProfile: gpu,
			}

			jobResp, err := apiClient.CreateJob(cmd.Context(), req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			_, results, err := pollAndReport(cmd.Context(), jobResp.JobID,
				fmt.Sprintf("Comparing engines for %s", model))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"job_id":  jobResp.JobID,
					"model":   model,
					"results": results,
				})
			}

			printComparisonTable(results)
			return nil
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "Model to benchmark (required)")
	cmd.Flags().StringVar(&enginesFlag, "engines", "", "Comma-separated engines, min 2 (required)")
	cmd.Flags().StringVar(&gpu, "gpu", "", "GPU profile (required)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 32, "Concurrent requests")
	cmd.Flags().IntVar(&promptTokens, "prompt-tokens", 512, "Prompt tokens")
	cmd.Flags().IntVar(&outputTokens, "output-tokens", 256, "Output tokens")
	cmd.Flags().IntVar(&requests, "requests", 200, "Total requests")
	return cmd
}

// pollAndReport polls jobID to completion with a progress bar labeled desc, then fetches
// and returns the final results. Shared by `benchmark run`, `benchmark compare`, and `run`.
func pollAndReport(ctx context.Context, jobID, desc string) (*jobs.Job, []jobs.Result, error) {
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(desc+"..."),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionOnCompletion(func() { fmt.Fprintln(os.Stderr) }),
	)

	finalJob, err := apiClient.PollJob(ctx, jobID, func(j jobs.Job) {
		bar.Describe(fmt.Sprintf("%s [%s]...", desc, string(j.State)))
		bar.Add(1) //nolint:errcheck
	})
	if err != nil {
		return nil, nil, err
	}
	bar.Finish() //nolint:errcheck

	if finalJob.State == jobs.StateFailed {
		return finalJob, nil, fmt.Errorf("benchmark failed: %s", finalJob.ErrorMsg)
	}

	results, err := apiClient.GetJobResults(ctx, jobID)
	if err != nil {
		return finalJob, nil, fmt.Errorf("fetching results: %w", err)
	}
	return finalJob, results, nil
}

// ── run ───────────────────────────────────────────────────────────────────────
// Doesn't tear down docker-compose (long-lived infra), but does stop a worker it
// spawned itself unless --keep-worker is set.

// runManifest is the JSON artifact written by --save, for later debugging.
type runManifest struct {
	JobID   string               `json:"job_id"`
	Model   string               `json:"model"`
	Engines []string             `json:"engines"`
	GPUHost string               `json:"gpu_host,omitempty"`
	Request cli.CreateJobRequest `json:"request"`
	State   string               `json:"state"`
	Results []jobs.Result        `json:"results"`
	SavedAt time.Time            `json:"saved_at"`
}

func newRunCmd() *cobra.Command {
	var (
		modelFlag          string
		enginesFlag        string
		gpu                string
		concurrency        int
		promptTokens       int
		outputTokens       int
		requests           int
		autoRoute          bool
		projectDir         string
		orchestratorFlag   string
		keepWorker         bool
		startupTimeoutSecs int
		workerTimeoutSecs  int
		gpuHost            string
		remoteDir          string
		savePath           string
	)

	cmd := &cobra.Command{
		Use:   "run [model]",
		Short: "Run a benchmark end-to-end with no manual setup (starts services and workers as needed)",
		Long: `run submits and executes a benchmark with no manual intervention required.

Unlike 'benchmark run', which assumes the InferBolt backing services and a Python worker
are already running, 'run' will:
  1. Start the docker-compose backing services if the gateway isn't reachable.
  2. Use (or auto-provision, in local dev) an API key if none is configured.
  3. Spawn a worker for the requested --gpu profile if none is registered — locally, or
     over SSH on --gpu-host if set.
  4. Submit the benchmark (multiple --engines run as a comparison), wait, print results.

Examples:
  inferbolt run my-model --engines mock --gpu cpu
  inferbolt run llama-3.1-8b-instant --engines sglang,vllm --gpu a100-80gb --save run.json
  inferbolt run llama-3.1-8b-instant --engines vllm --gpu a100-80gb --gpu-host ubuntu@10.0.0.5`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			model := modelFlag
			if len(args) == 1 {
				model = args[0]
			}
			if model == "" {
				return fmt.Errorf("model is required: `inferbolt run <model> ...`")
			}
			if !autoRoute && enginesFlag == "" {
				return fmt.Errorf("--engines is required unless --auto-route is set")
			}
			if gpu == "" {
				return fmt.Errorf("--gpu is required")
			}
			engines := splitCSV(enginesFlag)

			ctx := cmd.Context()

			cleanup, err := ensureStack(ctx, stackOptions{
				projectDir:      projectDir,
				orchestratorURL: orchestratorURLFrom(orchestratorFlag),
				gpuProfile:      gpu,
				gpuHost:         gpuHost,
				remoteDir:       remoteDir,
				keepWorker:      keepWorker,
				startupTimeout:  time.Duration(startupTimeoutSecs) * time.Second,
				workerTimeout:   time.Duration(workerTimeoutSecs) * time.Second,
			})
			if err != nil {
				return err
			}
			defer cleanup()

			req := cli.CreateJobRequest{
				Model:   model,
				Engines: engines,
				Workload: jobs.WorkloadConfig{
					Concurrency:  concurrency,
					PromptTokens: promptTokens,
					OutputTokens: outputTokens,
					NumRequests:  requests,
				},
				GPUProfile: gpu,
				AutoRoute:  autoRoute,
			}

			jobResp, err := apiClient.CreateJob(ctx, req)
			if err != nil {
				return fmt.Errorf("submit job: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Watch progress at %s/dashboard\n", cfg.ServerURL)

			engineLabel := strings.Join(engines, ",")
			if jobResp.RecommendedEngine != "" {
				engineLabel = jobResp.RecommendedEngine
			}

			finalJob, results, err := pollAndReport(ctx, jobResp.JobID,
				fmt.Sprintf("Benchmarking %s on %s", model, engineLabel))
			if err != nil {
				return err
			}

			if savePath != "" {
				manifest := runManifest{
					JobID: jobResp.JobID, Model: model, Engines: engines, GPUHost: gpuHost,
					Request: req, State: string(finalJob.State), Results: results, SavedAt: time.Now().UTC(),
				}
				data, merr := json.MarshalIndent(manifest, "", "  ")
				if merr == nil {
					merr = os.WriteFile(savePath, data, 0o644)
				}
				if merr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to save run manifest to %s: %v\n", savePath, merr)
				} else {
					fmt.Fprintf(os.Stderr, "Saved run manifest to %s\n", savePath)
				}
			}

			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"job_id":  jobResp.JobID,
					"model":   model,
					"state":   string(finalJob.State),
					"engines": engines,
					"results": results,
				})
			}

			if len(engines) > 1 {
				printComparisonTable(results)
			} else {
				printResultsTable(results)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&modelFlag, "model", "", "Model to benchmark (or pass as a positional argument)")
	cmd.Flags().StringVar(&enginesFlag, "engines", "", "Comma-separated engines, e.g. vllm,sglang (required unless --auto-route)")
	cmd.Flags().StringVar(&gpu, "gpu", "", "GPU profile, e.g. a100-80gb or cpu (required)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 32, "Number of concurrent requests (batch size)")
	cmd.Flags().IntVar(&promptTokens, "prompt-tokens", 512, "Prompt length in tokens")
	cmd.Flags().IntVar(&outputTokens, "output-tokens", 256, "Max output tokens")
	cmd.Flags().IntVar(&requests, "requests", 200, "Total number of requests")
	cmd.Flags().BoolVar(&autoRoute, "auto-route", false, "Auto-select best engine via classifier")
	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "Repo root containing docker-compose.yml and worker/")
	cmd.Flags().StringVar(&orchestratorFlag, "orchestrator", "", "Orchestrator URL for worker-availability checks (default http://localhost:8081)")
	cmd.Flags().BoolVar(&keepWorker, "keep-worker", false, "Leave a spawned worker process running after the benchmark finishes")
	cmd.Flags().IntVar(&startupTimeoutSecs, "startup-timeout", 120, "Seconds to wait for backing services to become healthy")
	cmd.Flags().StringVar(&gpuHost, "gpu-host", "", "SSH target (user@host) to run the worker on instead of locally")
	cmd.Flags().StringVar(&remoteDir, "remote-dir", "~/inferbolt", "Repo path on --gpu-host (must already have worker/ deps installed)")
	cmd.Flags().StringVar(&savePath, "save", "", "Write the full run manifest (request, state, results) as JSON to this path")
	cmd.Flags().IntVar(&workerTimeoutSecs, "worker-timeout", 90, "Seconds to wait for a spawned worker to register")
	return cmd
}

// stackOptions configures the bootstrap that `run` and `agent` share.
type stackOptions struct {
	projectDir      string
	orchestratorURL string
	gpuProfile      string
	gpuHost         string
	remoteDir       string
	keepWorker      bool
	startupTimeout  time.Duration
	workerTimeout   time.Duration
}

// ensureStack takes a machine from "nothing running" to "ready to submit work":
// it brings the docker-compose services up if the gateway is unreachable,
// resolves or bootstraps a credential, and spawns a worker for gpuProfile if
// none is registered. It sets the package-level cfg and apiClient.
//
// The returned cleanup stops only a worker this call started, and does nothing
// when --keep-worker is set or when a worker was already registered.
func ensureStack(ctx context.Context, o stackOptions) (func(), error) {
	noop := func() {}

	resolved, err := resolveOrBootstrapConfig(ctx, o.projectDir, o.startupTimeout)
	if err != nil {
		return nil, err
	}
	cfg = resolved
	apiClient = cfg.NewClient()

	orchClient := cli.NewOrchestratorClient(o.orchestratorURL)
	hasIdle, err := orchClient.HasIdleWorker(ctx, o.gpuProfile)
	if err != nil {
		return nil, fmt.Errorf("checking worker availability (is the orchestrator reachable at %s?): %w",
			o.orchestratorURL, err)
	}
	if hasIdle {
		return noop, nil
	}

	var stopWorker func()
	if o.gpuHost != "" {
		fmt.Fprintf(os.Stderr, "No idle worker for gpu_profile=%s — starting one on %s...\n", o.gpuProfile, o.gpuHost)
		stopWorker, err = spawnRemoteWorkerSSH(ctx, o.gpuHost, o.remoteDir, o.orchestratorURL, o.gpuProfile)
	} else {
		fmt.Fprintf(os.Stderr, "No idle worker for gpu_profile=%s — starting one locally...\n", o.gpuProfile)
		stopWorker, err = spawnLocalWorker(ctx, o.projectDir, o.orchestratorURL, o.gpuProfile)
	}
	if err != nil {
		return nil, err
	}

	cleanup := stopWorker
	if o.keepWorker {
		fmt.Fprintln(os.Stderr, "--keep-worker set: leaving the spawned worker running.")
		cleanup = noop
	}

	if err := waitForIdleWorker(ctx, orchClient, o.gpuProfile, o.workerTimeout); err != nil {
		cleanup()
		return nil, err
	}
	fmt.Fprintln(os.Stderr, "Worker registered.")
	return cleanup, nil
}

func orchestratorURLFrom(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("INFERBOLT_ORCHESTRATOR_URL"); v != "" {
		return v
	}
	return "http://localhost:8081"
}

// resolveOrBootstrapConfig loads the CLI config, bootstrapping a local dev API key
// (never for a non-local --server) if none is configured yet.
func resolveOrBootstrapConfig(ctx context.Context, projectDir string, startupTimeout time.Duration) (*cli.Config, error) {
	loaded, err := cli.Load()
	effectiveServer := serverFlag
	if effectiveServer == "" {
		if loaded != nil {
			effectiveServer = loaded.ServerURL
		} else {
			effectiveServer = cli.DefaultServerURL
		}
	}

	if err == nil {
		applyFlagOverrides(loaded)
		if err := ensureServerHealthy(ctx, effectiveServer, projectDir, startupTimeout); err != nil {
			return nil, err
		}
		return loaded, nil
	}

	if !errors.Is(err, cli.ErrAPIKeyNotConfigured) {
		return nil, err
	}
	if !isLocalServerURL(effectiveServer) {
		return nil, fmt.Errorf("%w (and --server points at a non-local address, so a credential can't be auto-provisioned)", err)
	}

	if err := ensureServerHealthy(ctx, effectiveServer, projectDir, startupTimeout); err != nil {
		return nil, err
	}

	tokenPath := filepath.Join(projectDir, ".inferbolt-dev", "dev-token")
	token, err := waitForDevToken(ctx, tokenPath, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("no API key configured and could not read a bootstrapped dev token from %s: %w\n"+
			"run 'inferbolt configure' to set one manually", tokenPath, err)
	}

	newCfg := &cli.Config{
		ServerURL: effectiveServer,
		APIKey:    token,
		OutputFmt: "table",
	}
	applyFlagOverrides(newCfg)
	if err := cli.Save(newCfg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: bootstrapped a dev API key but failed to save it to config: %v\n", err)
	} else {
		fmt.Fprintln(os.Stderr, "Bootstrapped a local dev API key and saved it to ~/.inferbolt/config.yaml")
	}
	return newCfg, nil
}

func applyFlagOverrides(c *cli.Config) {
	if serverFlag != "" {
		c.ServerURL = serverFlag
	}
	if apiKeyFlag != "" {
		c.APIKey = apiKeyFlag
	}
	if outputFlag != "" {
		c.OutputFmt = outputFlag
	}
}

// isLocalServerURL reports whether rawURL points at loopback.
func isLocalServerURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func ensureServerHealthy(ctx context.Context, serverURL, projectDir string, startupTimeout time.Duration) error {
	probe := cli.NewClient(serverURL, "")
	if _, err := probe.Health(ctx); err == nil {
		return nil // already up
	}

	composeFile := filepath.Join(projectDir, "docker-compose.yml")
	if _, err := os.Stat(composeFile); err != nil {
		return fmt.Errorf("gateway at %s is unreachable and no docker-compose.yml found at %s "+
			"(pass --project-dir if you're not running from the repo root): %w", serverURL, composeFile, err)
	}

	fmt.Fprintf(os.Stderr, "Gateway at %s is unreachable — starting backing services (docker compose up -d)...\n", serverURL)
	if err := runComposeUp(ctx, composeFile); err != nil {
		return err
	}

	deadline := time.Now().Add(startupTimeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("gateway did not become healthy within %s — check `docker compose -f %s logs gateway`",
				startupTimeout, composeFile)
		}
		if _, err := probe.Health(ctx); err == nil {
			fmt.Fprintln(os.Stderr, "Backing services are up.")
			return nil
		}
		time.Sleep(2 * time.Second)
	}
}

func runComposeUp(ctx context.Context, composeFile string) error {
	var cmd *exec.Cmd
	if _, err := exec.LookPath("docker"); err == nil {
		cmd = exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "up", "-d")
	} else if _, err := exec.LookPath("docker-compose"); err == nil {
		cmd = exec.CommandContext(ctx, "docker-compose", "-f", composeFile, "up", "-d")
	} else {
		return fmt.Errorf("neither `docker` nor `docker-compose` was found on PATH; install Docker or start the backing services manually")
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up failed: %w\n%s", err, out.String())
	}
	return nil
}

// waitForDevToken polls for the gateway's bootstrapped dev-token file to appear.
func waitForDevToken(ctx context.Context, path string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			if token := strings.TrimSpace(string(data)); token != "" {
				return token, nil
			}
		}
		if time.Now().After(deadline) {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// spawnLocalWorker starts worker/main.py as a child process and returns a func to stop it.
func spawnLocalWorker(ctx context.Context, projectDir, orchestratorURL, gpuProfile string) (stop func(), err error) {
	workerPort, err := cli.FreeTCPPort()
	if err != nil {
		return nil, fmt.Errorf("find a free port for the worker: %w", err)
	}

	var name string
	var args []string
	if _, lookErr := exec.LookPath("uv"); lookErr == nil {
		name, args = "uv", []string{"run", "python", "-m", "worker.main"}
	} else if _, lookErr := exec.LookPath("python"); lookErr == nil {
		name, args = "python", []string{"-m", "worker.main"}
	} else if _, lookErr := exec.LookPath("python3"); lookErr == nil {
		name, args = "python3", []string{"-m", "worker.main"}
	} else {
		return nil, fmt.Errorf("none of uv, python, or python3 found on PATH; install one to let `run` spawn a worker, " +
			"or start worker/main.py yourself and re-run")
	}

	c := exec.CommandContext(ctx, name, args...)
	c.Dir = projectDir
	c.Env = append(os.Environ(),
		"ORCHESTRATOR_URL="+orchestratorURL,
		"PORT="+strconv.Itoa(workerPort),
		"WORKER_URL=http://127.0.0.1:"+strconv.Itoa(workerPort),
		"GPU_PROFILE="+gpuProfile,
	)

	stop, err = startManagedProcess(c, fmt.Sprintf("local worker (%s %s)", name, strings.Join(args, " ")))
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "Started local worker (pid %d) on port %d for gpu_profile=%s\n", c.Process.Pid, workerPort, gpuProfile)
	return stop, nil
}

// spawnRemoteWorkerSSH starts worker/main.py on gpuHost over SSH, using one SSH session
// that both runs the remote command and carries two tunnels so the orchestrator (running
// locally) and the remote worker can reach each other despite being on different networks:
//   - -L localPort:127.0.0.1:workerPort  — lets the local orchestrator reach the remote
//     worker via http://host.docker.internal:localPort (from inside its container).
//   - -R remotePort:127.0.0.1:orchestratorPort — lets the remote worker reach back to the
//     local orchestrator via http://localhost:remotePort.
//
// Killing the ssh process stops both the tunnel and (SSH's default behavior) the remote
// command. remoteDir must already contain the repo with worker/ dependencies installed —
// this does not provision the remote environment, only starts what's already there.
func spawnRemoteWorkerSSH(ctx context.Context, gpuHost, remoteDir, orchestratorURL, gpuProfile string) (stop func(), err error) {
	if _, lookErr := exec.LookPath("ssh"); lookErr != nil {
		return nil, fmt.Errorf("`ssh` not found on PATH; required for --gpu-host")
	}

	orchestratorPort, err := portFromURL(orchestratorURL)
	if err != nil {
		return nil, fmt.Errorf("parse --orchestrator port: %w", err)
	}
	localPort, err := cli.FreeTCPPort()
	if err != nil {
		return nil, fmt.Errorf("find a free local port: %w", err)
	}
	remotePort, err := cli.FreeTCPPort()
	if err != nil {
		return nil, fmt.Errorf("find a free port for the reverse tunnel: %w", err)
	}
	workerPort, err := cli.FreeTCPPort()
	if err != nil {
		return nil, fmt.Errorf("find a free port for the remote worker: %w", err)
	}

	// WORKER_URL is what the worker registers with the orchestrator and gets dispatched to —
	// it must resolve from *inside the orchestrator's container*, not from the remote host,
	// hence host.docker.internal:localPort (routed back through the -L tunnel) rather than
	// the worker's own loopback address.
	remoteCmd := fmt.Sprintf(
		"cd %s && ORCHESTRATOR_URL=http://localhost:%d PORT=%d WORKER_URL=http://host.docker.internal:%d GPU_PROFILE=%s "+
			"(uv run python -m worker.main || python3 -m worker.main || python -m worker.main)",
		remoteDir, remotePort, workerPort, localPort, gpuProfile,
	)
	c := exec.CommandContext(ctx, "ssh",
		"-L", fmt.Sprintf("%d:127.0.0.1:%d", localPort, workerPort),
		"-R", fmt.Sprintf("%d:127.0.0.1:%d", remotePort, orchestratorPort),
		gpuHost, remoteCmd,
	)

	stop, err = startManagedProcess(c, "ssh to "+gpuHost)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "Started remote worker on %s:%s (pid %d, tunneled via localhost:%d) for gpu_profile=%s\n",
		gpuHost, remoteDir, c.Process.Pid, localPort, gpuProfile)
	return stop, nil
}

// portFromURL extracts the port from a URL like http://localhost:8081.
func portFromURL(rawURL string) (int, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(u.Port())
}

// startManagedProcess starts c with combined output captured, returning a stop func and
// surfacing an immediate crash (bad deps, port in use) instead of waiting out a caller's
// full readiness timeout.
func startManagedProcess(c *exec.Cmd, label string) (stop func(), err error) {
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out

	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", label, err)
	}

	exited := make(chan error, 1)
	go func() { exited <- c.Wait() }()

	stop = func() {
		select {
		case <-exited:
			return
		default:
		}
		if c.Process != nil {
			_ = c.Process.Kill()
		}
		<-exited
	}

	select {
	case werr := <-exited:
		return nil, fmt.Errorf("%s exited immediately (%v):\n%s", label, werr, out.String())
	case <-time.After(500 * time.Millisecond):
	}
	return stop, nil
}

func waitForIdleWorker(ctx context.Context, orchClient *cli.OrchestratorClient, gpuProfile string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		hasIdle, err := orchClient.HasIdleWorker(ctx, gpuProfile)
		if err == nil && hasIdle {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("worker did not register with the orchestrator within %s for gpu_profile=%s", timeout, gpuProfile)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// ── jobs ──────────────────────────────────────────────────────────────────────

func newJobsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "jobs", Short: "Manage benchmark jobs"}
	cmd.AddCommand(newJobsListCmd())
	cmd.AddCommand(newJobsGetCmd())
	cmd.AddCommand(newJobsCancelCmd())
	return cmd
}

func newJobsListCmd() *cobra.Command {
	var (
		state string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List benchmark jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			jobList, err := apiClient.ListJobs(cmd.Context(), state, limit)
			if err != nil {
				return err
			}
			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(jobList)
			}
			tbl := tablewriter.NewWriter(os.Stdout)
			tbl.SetHeader([]string{"JOB ID", "MODEL", "STATE", "ENGINES", "CREATED"})
			tbl.SetBorder(false)
			for _, j := range jobList {
				id := j.ID
				if len(id) > 8 {
					id = id[:8]
				}
				tbl.Append([]string{
					id,
					j.Model,
					string(j.State),
					strings.Join(j.Engines, ","),
					j.CreatedAt.Format("2006-01-02 15:04"),
				})
			}
			tbl.Render()
			return nil
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "Filter by state (pending, running, completed, failed)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum jobs to return")
	return cmd
}

func newJobsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <jobID>",
		Short: "Get job details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			job, err := apiClient.GetJob(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(job)
			}
			tbl := tablewriter.NewWriter(os.Stdout)
			tbl.SetHeader([]string{"FIELD", "VALUE"})
			tbl.SetBorder(false)
			tbl.Append([]string{"ID", job.ID})
			tbl.Append([]string{"MODEL", job.Model})
			tbl.Append([]string{"STATE", string(job.State)})
			tbl.Append([]string{"ENGINES", strings.Join(job.Engines, ", ")})
			tbl.Append([]string{"GPU PROFILE", job.GPUProfile})
			tbl.Append([]string{"CREATED", job.CreatedAt.Format(time.RFC3339)})
			tbl.Append([]string{"UPDATED", job.UpdatedAt.Format(time.RFC3339)})
			if job.ErrorMsg != "" {
				tbl.Append([]string{"ERROR", job.ErrorMsg})
			}
			tbl.Render()
			return nil
		},
	}
}

func newJobsCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <jobID>",
		Short: "Cancel a non-terminal job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.CancelJob(cmd.Context(), args[0]); err != nil {
				return err
			}
			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(map[string]bool{"cancelled": true})
			}
			fmt.Printf("Job %s cancelled.\n", args[0])
			return nil
		},
	}
}

// ── metrics ───────────────────────────────────────────────────────────────────

func newMetricsCmd() *cobra.Command {
	var (
		engine string
		model  string
		since  string
	)
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Query benchmark metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			sinceTime, err := parseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since value %q: %w", since, err)
			}

			results, err := apiClient.GetMetrics(cmd.Context(), engine, model, sinceTime)
			if err != nil {
				return err
			}
			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(results)
			}
			printResultsTable(results)
			return nil
		},
	}
	cmd.Flags().StringVar(&engine, "engine", "", "Engine name")
	cmd.Flags().StringVar(&model, "model", "", "Model name")
	cmd.Flags().StringVar(&since, "since", "24h", "Time window (e.g. 24h, 7d, 1h)")
	return cmd
}

// ── route ─────────────────────────────────────────────────────────────────────

func newRouteCmd() *cobra.Command {
	var (
		promptTokens      int
		outputTokens      int
		concurrency       int
		structuredOutput  bool
		toolCalls         bool
		sharedPrefixRatio float64
	)
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Classify a workload and get engine recommendation",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := apiClient.ClassifyWorkload(cmd.Context(), router.ClassificationInput{
				PromptTokens:      promptTokens,
				OutputTokens:      outputTokens,
				Concurrency:       concurrency,
				StructuredOutput:  structuredOutput,
				ToolCalls:         toolCalls,
				SharedPrefixRatio: sharedPrefixRatio,
			})
			if err != nil {
				return err
			}
			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(result)
			}
			fmt.Printf("Workload type:       %s\n", result.WorkloadType)
			fmt.Printf("Recommended engine:  %s\n", result.RecommendedEngine)
			fmt.Printf("Confidence:          %.0f%%\n", result.Confidence*100)
			fmt.Printf("Reasoning:           %s\n", result.Reasoning)
			return nil
		},
	}
	cmd.Flags().IntVar(&promptTokens, "prompt-tokens", 512, "Prompt length in tokens")
	cmd.Flags().IntVar(&outputTokens, "output-tokens", 256, "Max output tokens")
	cmd.Flags().IntVar(&concurrency, "concurrency", 32, "Expected concurrent requests")
	cmd.Flags().BoolVar(&structuredOutput, "structured-output", false, "Requires structured/JSON output")
	cmd.Flags().BoolVar(&toolCalls, "tool-calls", false, "Uses tool/function calls")
	cmd.Flags().Float64Var(&sharedPrefixRatio, "shared-prefix-ratio", 0.0, "Fraction of shared prompt prefix across requests")
	return cmd
}

// ── configure ─────────────────────────────────────────────────────────────────

func newConfigureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "configure",
		Short: "Configure InferBolt CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			sc := bufio.NewScanner(os.Stdin)

			fmt.Print("Server URL [http://localhost:8080]: ")
			sc.Scan()
			serverURL := strings.TrimSpace(sc.Text())
			if serverURL == "" {
				serverURL = "http://localhost:8080"
			}

			fmt.Print("API key: ")
			sc.Scan()
			apiKey := strings.TrimSpace(sc.Text())

			fmt.Print("Tenant ID (optional): ")
			sc.Scan()
			tenantID := strings.TrimSpace(sc.Text())

			c := &cli.Config{
				ServerURL: serverURL,
				APIKey:    apiKey,
				TenantID:  tenantID,
				OutputFmt: "table",
			}
			if err := cli.Save(c); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Println("Configuration saved to ~/.inferbolt/config.yaml")
			return nil
		},
	}
}

// ── version ───────────────────────────────────────────────────────────────────

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL := "http://localhost:8080"
			if cfg != nil {
				serverURL = cfg.ServerURL
			} else if serverFlag != "" {
				serverURL = serverFlag
			}

			fmt.Printf("inferbolt version %s\n", version)
			fmt.Printf("server: %s\n", serverURL)

			if apiClient != nil {
				health, err := apiClient.Health(cmd.Context())
				if err != nil {
					fmt.Printf("server status: unreachable (%v)\n", err)
					return nil
				}
				fmt.Printf("server version: %s\n", health.Version)
				fmt.Printf("postgres: %s\n", health.Postgres)
			}
			return nil
		},
	}
}

// ── admin ─────────────────────────────────────────────────────────────────────

func newAdminCmd() *cobra.Command {
	adminCmd := &cobra.Command{Use: "admin", Short: "Administrative commands"}
	apiKeysCmd := &cobra.Command{Use: "apikeys", Short: "Manage API keys"}
	apiKeysCmd.AddCommand(newAdminAPIKeysCreateCmd())
	adminCmd.AddCommand(apiKeysCmd)
	return adminCmd
}

func newAdminAPIKeysCreateCmd() *cobra.Command {
	var (
		tenantID   string
		scopesFlag string
		expiryDays int
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if tenantID == "" {
				return fmt.Errorf("--tenant-id is required")
			}
			if scopesFlag == "" {
				return fmt.Errorf("--scopes is required")
			}
			scopes := splitCSV(scopesFlag)

			resp, err := apiClient.CreateAPIKey(cmd.Context(), tenantID, scopes, expiryDays)
			if err != nil {
				return err
			}

			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(resp)
			}

			fmt.Println()
			fmt.Println("⚠  Store this token securely — it will not be shown again")
			fmt.Println()
			fmt.Printf("Token:      %s\n", resp.Token)
			fmt.Printf("Expires at: %s\n", resp.ExpiresAt.Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&tenantID, "tenant-id", "", "Tenant ID (required)")
	cmd.Flags().StringVar(&scopesFlag, "scopes", "", "Comma-separated scopes (required)")
	cmd.Flags().IntVar(&expiryDays, "expiry-days", 30, "Days until expiry (1–365)")
	return cmd
}

// ── display helpers ───────────────────────────────────────────────────────────

func printResultsTable(results []jobs.Result) {
	tbl := tablewriter.NewWriter(os.Stdout)
	tbl.SetHeader([]string{"ENGINE", "TTFT P50", "TTFT P99", "TOK/S", "GPU MEM", "COST/MTOK", "KV HIT"})
	tbl.SetBorder(false)
	for _, r := range results {
		tbl.Append([]string{
			r.Engine,
			fmt.Sprintf("%.1f ms", r.TTFTP50Ms),
			fmt.Sprintf("%.1f ms", r.TTFTP99Ms),
			fmt.Sprintf("%.0f", r.TokPerSec),
			fmt.Sprintf("%d MB", r.GPUMemMB),
			fmt.Sprintf("$%.4f", r.CostPerMTok),
			fmt.Sprintf("%.1f%%", r.KVCacheHit*100),
		})
	}
	tbl.Render()
}

func printComparisonTable(results []jobs.Result) {
	if len(results) == 0 {
		fmt.Println("No results available.")
		return
	}

	// Find best/worst indices for each numeric metric
	type metric struct {
		higherBetter bool
		vals         []float64
	}
	metrics := []metric{
		{false, make([]float64, len(results))}, // TTFT P50 - lower better
		{false, make([]float64, len(results))}, // TTFT P99 - lower better
		{true, make([]float64, len(results))},  // TOK/S - higher better
		{false, make([]float64, len(results))}, // GPU MEM - lower better
		{false, make([]float64, len(results))}, // COST/MTOK - lower better
		{true, make([]float64, len(results))},  // KV HIT - higher better
	}
	for i, r := range results {
		metrics[0].vals[i] = r.TTFTP50Ms
		metrics[1].vals[i] = r.TTFTP99Ms
		metrics[2].vals[i] = r.TokPerSec
		metrics[3].vals[i] = float64(r.GPUMemMB)
		metrics[4].vals[i] = r.CostPerMTok
		metrics[5].vals[i] = r.KVCacheHit
	}

	bestIdx := make([]int, len(metrics))
	worstIdx := make([]int, len(metrics))
	for m, met := range metrics {
		best := 0
		worst := 0
		for i, v := range met.vals {
			if met.higherBetter {
				if v > met.vals[best] {
					best = i
				}
				if v < met.vals[worst] {
					worst = i
				}
			} else {
				if v < met.vals[best] {
					best = i
				}
				if v > met.vals[worst] {
					worst = i
				}
			}
		}
		bestIdx[m] = best
		worstIdx[m] = worst
	}

	tbl := tablewriter.NewWriter(os.Stdout)
	tbl.SetHeader([]string{"ENGINE", "TTFT P50", "TTFT P99", "TOK/S", "GPU MEM", "COST/MTOK", "KV HIT"})
	tbl.SetBorder(false)

	for i, r := range results {
		row := []string{
			r.Engine,
			fmt.Sprintf("%.1f ms", r.TTFTP50Ms),
			fmt.Sprintf("%.1f ms", r.TTFTP99Ms),
			fmt.Sprintf("%.0f", r.TokPerSec),
			fmt.Sprintf("%d MB", r.GPUMemMB),
			fmt.Sprintf("$%.4f", r.CostPerMTok),
			fmt.Sprintf("%.1f%%", r.KVCacheHit*100),
		}
		colors := make([]tablewriter.Colors, 8) // ENGINE col + 6 metric cols + padding
		for col := 0; col < len(metrics); col++ {
			switch i {
			case bestIdx[col]:
				colors[col+1] = tablewriter.Colors{tablewriter.FgGreenColor, tablewriter.Bold}
			case worstIdx[col]:
				colors[col+1] = tablewriter.Colors{tablewriter.FgRedColor}
			}
		}
		tbl.Rich(row, colors)
	}
	tbl.Render()

	// Recommendation: engine with highest tok/s
	best := results[0]
	for _, r := range results[1:] {
		if r.TokPerSec > best.TokPerSec {
			best = r
		}
	}
	fmt.Printf("\nRecommendation: use %s — highest throughput at %.0f tok/s ($%.4f/MTok)\n",
		best.Engine, best.TokPerSec, best.CostPerMTok)
}

// ── utility helpers ───────────────────────────────────────────────────────────

func isJSON() bool {
	if outputFlag == "json" {
		return true
	}
	if cfg != nil && cfg.OutputFmt == "json" {
		return true
	}
	return false
}

// splitCSV parses a comma-separated list, tolerating an optional wrapping
// "[...]" (e.g. --engines "[sglang, vllm]") and surrounding whitespace.
func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseSince(s string) (time.Time, error) {
	// Handle Xd format (e.g. "7d")
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil && days > 0 {
			return time.Now().Add(-time.Duration(days) * 24 * time.Hour), nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-d), nil
}
