package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/inferbolthq/inferbolt/internal/agent"
	"github.com/inferbolthq/inferbolt/internal/cli"
	"github.com/inferbolthq/inferbolt/internal/jobs"
)

func newAgentCmd() *cobra.Command {
	var (
		model        string
		gpu          string
		enginesFlag  string
		concurrency  int
		promptTokens int
		outputTokens int
		requests     int
		maxTrials    int
		maxDuration  time.Duration
		plannerModel string
	)

	cmd := &cobra.Command{
		Use:   "agent <goal>",
		Short: "Run a goal-directed optimization campaign",
		Long: `Run a goal-directed optimization campaign.

Give the agent a goal in plain English. It plans a sequence of benchmarks,
submits them through the ordinary API, reads the measurements back, and
recommends an engine and configuration — citing the trials that support it.

The model, GPU profile and workload are fixed for the whole campaign so that
trials stay comparable; the agent varies the engine and its configuration.

Every benchmark occupies a GPU worker for minutes, so a campaign runs against
a hard trial and wall-clock budget. Planning uses the Anthropic API and needs
ANTHROPIC_API_KEY (or a configured Anthropic CLI profile) in the environment.`,
		Example: `  inferbolt agent "cheapest engine for chat under 200ms p99 TTFT" \
    --model meta-llama/Llama-3.1-8B --gpu a100-80gb --engines vllm,sglang

  # no GPU required — exercises the whole loop against the mock engine
  inferbolt agent "find the best throughput config" \
    --model test-model --gpu cpu --engines mock --max-trials 4`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			campaign := agent.Campaign{
				Goal:       args[0],
				Model:      model,
				GPUProfile: gpu,
				Engines:    splitCSV(enginesFlag),
				Workload: jobs.WorkloadConfig{
					Concurrency:  concurrency,
					PromptTokens: promptTokens,
					OutputTokens: outputTokens,
					NumRequests:  requests,
				},
				Budget: agent.Budget{
					MaxTrials:   maxTrials,
					MaxDuration: maxDuration,
					MaxTurns:    agent.DefaultBudget().MaxTurns,
				},
			}

			platform := cli.NewAgentPlatform(apiClient)

			opts := []agent.Option{agent.WithModel(plannerModel)}
			if !isJSON() {
				opts = append(opts, agent.WithObserver(renderEvent))
				printCampaignHeader(campaign, plannerModel)
			}

			a, err := agent.New(platform, campaign, opts...)
			if err != nil {
				return err
			}

			report, runErr := a.Run(cmd.Context())

			if isJSON() {
				if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
					return err
				}
				return runErr
			}

			printReport(report)
			return runErr
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "Model to optimize for (required)")
	cmd.Flags().StringVar(&gpu, "gpu", "", "GPU profile to benchmark on, e.g. a100-80gb or cpu (required)")
	cmd.Flags().StringVar(&enginesFlag, "engines", "vllm,sglang", "Candidate engines the agent may try")
	cmd.Flags().IntVar(&concurrency, "concurrency", 32, "Number of concurrent requests")
	cmd.Flags().IntVar(&promptTokens, "prompt-tokens", 512, "Prompt length in tokens")
	cmd.Flags().IntVar(&outputTokens, "output-tokens", 256, "Max output tokens")
	cmd.Flags().IntVar(&requests, "requests", 200, "Total number of requests per trial")
	cmd.Flags().IntVar(&maxTrials, "max-trials", agent.DefaultBudget().MaxTrials, "Maximum benchmarks the agent may run")
	cmd.Flags().DurationVar(&maxDuration, "max-duration", agent.DefaultBudget().MaxDuration, "Wall-clock limit for the campaign")
	cmd.Flags().StringVar(&plannerModel, "planner-model", agent.DefaultModel, "Anthropic model used to plan the campaign")

	cmd.MarkFlagRequired("model") //nolint:errcheck
	cmd.MarkFlagRequired("gpu")   //nolint:errcheck

	return cmd
}

func printCampaignHeader(c agent.Campaign, planner string) {
	fmt.Printf("InferBolt agent — %s on %s\n", c.Model, c.GPUProfile)
	fmt.Printf("Goal:     %s\n", c.Goal)
	fmt.Printf("Engines:  %s\n", strings.Join(c.Engines, ", "))
	fmt.Printf("Workload: %d concurrent, %d/%d tokens, %d requests\n",
		c.Workload.Concurrency, c.Workload.PromptTokens, c.Workload.OutputTokens, c.Workload.NumRequests)
	fmt.Printf("Budget:   %d trials, %s (planner: %s)\n\n", c.Budget.MaxTrials, c.Budget.MaxDuration, planner)
}

// renderEvent streams campaign progress. A campaign against real hardware runs
// for many minutes, so it prints as it goes rather than only at the end.
func renderEvent(e agent.Event) {
	switch e.Kind {
	case agent.EventPlan:
		fmt.Printf("[plan]   %s\n", indentWrap(e.Text))
	case agent.EventThinking:
		fmt.Printf("[think]  %s\n", indentWrap(e.Text))
	case agent.EventLookup:
		fmt.Printf("[look]   %s\n", e.Text)
	case agent.EventTrialStart:
		fmt.Printf("[run]    %-8s %-24s %s\n", e.Trial.Engine, configSummary(e.Trial.EngineConfig), e.Text)
	case agent.EventTrialDone:
		fmt.Printf("[ok]     %-8s %-24s %.0f tok/s  %.0fms p99 TTFT  $%.4f/Mtok\n",
			e.Trial.Engine, configSummary(e.Trial.EngineConfig),
			e.Result.TokPerSec, e.Result.TTFTP99Ms, e.Result.CostPerMTok)
	case agent.EventTrialFailed:
		fmt.Printf("[fail]   %-8s %-24s %s\n", e.Trial.Engine, configSummary(e.Trial.EngineConfig), e.Text)
	case agent.EventWarn:
		fmt.Printf("[warn]   %s\n", e.Text)
	case agent.EventDone:
		fmt.Printf("[done]   campaign complete (%s)\n", e.Elapsed.Round(time.Second))
	}
}

func printReport(r *agent.Report) {
	rec := r.Recommendation
	fmt.Println()

	if rec == nil {
		fmt.Println("No recommendation: the campaign produced no usable measurement.")
		printCampaignFooter(r)
		return
	}

	fmt.Printf("Recommendation: %s %s\n", rec.Engine, configSummary(rec.EngineConfig))
	if rec.Fallback {
		fmt.Println("  (selected mechanically — the planner did not conclude)")
	}
	if m := rec.Measured; m != nil {
		fmt.Printf("  throughput   %.0f tok/s\n", m.TokPerSec)
		fmt.Printf("  p99 TTFT     %.0f ms\n", m.TTFTP99Ms)
		fmt.Printf("  cost         $%.4f / Mtok\n", m.CostPerMTok)
		if m.ErrorRate > 0 {
			fmt.Printf("  error rate   %.2f%%\n", m.ErrorRate*100)
		}
	}
	fmt.Printf("  confidence   %s\n", rec.Confidence)
	fmt.Printf("\n%s\n", indentWrap(rec.Reasoning))
	if rec.RunnerUp != "" {
		fmt.Printf("\nRunner-up: %s\n", indentWrap(rec.RunnerUp))
	}
	if rec.Caveats != "" {
		fmt.Printf("\nCaveats: %s\n", indentWrap(rec.Caveats))
	}

	printCampaignFooter(r)
}

func printCampaignFooter(r *agent.Report) {
	measured := 0
	for _, t := range r.Trials {
		if !t.Reused {
			measured++
		}
	}

	fmt.Printf("\n%d benchmark%s in %s.\n",
		measured, plural(measured), r.Elapsed.Round(time.Second))

	tokens := r.Usage.InputTokens + r.Usage.OutputTokens + r.Usage.CacheReadTokens
	line := fmt.Sprintf("Planning: %s, %d turns, %s tokens",
		r.PlannerModel, r.Usage.Turns, humanTokens(tokens))
	if cost, known := r.Usage.EstimateUSD(r.PlannerModel); known {
		line += fmt.Sprintf(" (~$%.2f)", cost)
	}
	fmt.Println(line + ".")
}

// configSummary renders an engine config compactly. An empty config means the
// engine's own defaults were used, which is worth showing explicitly.
func configSummary(c jobs.EngineConfig) string {
	var parts []string
	if c.Quantization != "" {
		parts = append(parts, c.Quantization)
	}
	if c.TensorParallel > 0 {
		parts = append(parts, fmt.Sprintf("TP%d", c.TensorParallel))
	}
	if c.MaxBatchSize > 0 {
		parts = append(parts, fmt.Sprintf("bs%d", c.MaxBatchSize))
	}
	if c.MaxModelLen > 0 {
		parts = append(parts, fmt.Sprintf("len%d", c.MaxModelLen))
	}
	if len(parts) == 0 {
		return "engine defaults"
	}
	return strings.Join(parts, " ")
}

// indentWrap wraps prose to a readable width, aligned under the label column.
func indentWrap(s string) string {
	const width = 76
	const indent = "         "

	var out strings.Builder
	col := 0
	for i, word := range strings.Fields(s) {
		switch {
		case i == 0:
			out.WriteString(word)
			col = len(word)
		case col+1+len(word) > width:
			out.WriteString("\n" + indent + word)
			col = len(word)
		default:
			out.WriteString(" " + word)
			col += 1 + len(word)
		}
	}
	return out.String()
}

func humanTokens(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
