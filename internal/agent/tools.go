package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/router"
)

// Tool names. The set is deliberately small and closed: four reads and one
// action. The agent has no shell, no filesystem, and no way to reach the
// platform except through these.
const (
	toolListWorkers      = "list_workers"
	toolClassifyWorkload = "classify_workload"
	toolHistorical       = "historical_results"
	toolRunBenchmark     = "run_benchmark"
	toolRecommend        = "submit_recommendation"
)

func toolDefinitions(engines []string) []anthropic.ToolUnionParam {
	engineEnum := make([]any, len(engines))
	for i, e := range engines {
		engineEnum[i] = e
	}

	quantEnum := []any{"", "fp8", "int8", "int4", "gptq", "awq"}

	engineConfigProps := map[string]any{
		"quantization": map[string]any{
			"type":        "string",
			"enum":        quantEnum,
			"description": "Weight quantization. Empty string means full precision.",
		},
		"tensor_parallel": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     8,
			"description": "GPUs to shard the model across. Omit for the engine default (1).",
		},
		"max_batch_size": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     4096,
			"description": "Maximum batch size. Larger trades time-to-first-token for throughput.",
		},
		"max_model_len": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"maximum":     1048576,
			"description": "Context length to configure the engine with. Omit for the engine default.",
		},
		"gpu_memory_utilization": map[string]any{
			"type":        "number",
			"description": "Fraction of GPU memory the engine may claim, between 0 and 1. Omit for the engine default.",
		},
	}

	tools := []anthropic.ToolParam{
		{
			Name: toolListWorkers,
			Description: anthropic.String(
				"List registered benchmark workers and their GPU profiles and status. " +
					"A trial can only run where an idle worker exists for the campaign's GPU profile."),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: map[string]any{}},
		},
		{
			Name: toolClassifyWorkload,
			Description: anthropic.String(
				"Ask the deterministic rules-based router which engine suits a workload shape. " +
					"Returns a prior with reasoning, not a measurement."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"structured_output":   map[string]any{"type": "boolean", "description": "Workload requires JSON-schema-constrained decoding."},
					"tool_calls":          map[string]any{"type": "boolean", "description": "Workload involves function calling."},
					"shared_prefix_ratio": map[string]any{"type": "number", "description": "Fraction of the prompt shared across requests, 0 to 1. High values favour prefix-caching engines."},
				},
			},
		},
		{
			Name: toolHistorical,
			Description: anthropic.String(
				"Look up past benchmark results for an engine and this campaign's model. " +
					"Use this before spending a trial on something already measured."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"engine":      map[string]any{"type": "string", "enum": engineEnum},
					"since_hours": map[string]any{"type": "integer", "minimum": 1, "maximum": 8760, "description": "How far back to look. Defaults to 168 (one week)."},
				},
				Required: []string{"engine"},
			},
		},
		{
			Name: toolRunBenchmark,
			Description: anthropic.String(
				"Run one benchmark: this campaign's model and workload, on the engine and " +
					"configuration you specify. Blocks until the job finishes and returns the " +
					"measured metrics. This is the only tool that consumes trial budget and the " +
					"only one with side effects."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: mergeProps(map[string]any{
					"engine": map[string]any{"type": "string", "enum": engineEnum},
					"hypothesis": map[string]any{
						"type":        "string",
						"description": "What you expect this trial to tell you, and why it is worth the budget.",
					},
				}, engineConfigProps),
				Required: []string{"engine", "hypothesis"},
			},
		},
		{
			Name: toolRecommend,
			Description: anthropic.String(
				"Conclude the campaign with your recommended engine and configuration. " +
					"Call this exactly once, when you have enough evidence."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: mergeProps(map[string]any{
					"engine": map[string]any{"type": "string", "enum": engineEnum},
					"reasoning": map[string]any{
						"type":        "string",
						"description": "Why this configuration wins for the stated goal, citing the measurements that support it.",
					},
					"runner_up": map[string]any{
						"type":        "string",
						"description": "The next best configuration and what it would cost or gain to choose it instead.",
					},
					"confidence": map[string]any{
						"type":        "string",
						"enum":        []any{"high", "medium", "low"},
						"description": "low when the campaign ran out of budget before the picture was clear, or when results were close enough to be noise.",
					},
					"caveats": map[string]any{
						"type":        "string",
						"description": "Anything that qualifies the result: an unmet constraint, an untested dimension, a suspicious measurement.",
					},
				}, engineConfigProps),
				Required: []string{"engine", "reasoning", "confidence"},
			},
		},
	}

	out := make([]anthropic.ToolUnionParam, len(tools))
	for i := range tools {
		out[i] = anthropic.ToolUnionParam{OfTool: &tools[i]}
	}
	return out
}

func mergeProps(base, extra map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	return merged
}

// engineConfigInput is the flattened config the model supplies. It is flat
// rather than nested because a flat schema is markedly easier for a model to
// fill in correctly, and it maps one-to-one onto jobs.EngineConfig.
type engineConfigInput struct {
	Quantization         string  `json:"quantization"`
	TensorParallel       int     `json:"tensor_parallel"`
	MaxBatchSize         int     `json:"max_batch_size"`
	MaxModelLen          int     `json:"max_model_len"`
	GPUMemoryUtilization float64 `json:"gpu_memory_utilization"`
}

func (e engineConfigInput) toJobs() jobs.EngineConfig {
	return jobs.EngineConfig{
		Quantization:         e.Quantization,
		TensorParallel:       e.TensorParallel,
		MaxBatchSize:         e.MaxBatchSize,
		MaxModelLen:          e.MaxModelLen,
		GPUMemoryUtilization: e.GPUMemoryUtilization,
	}
}

// dispatchTool executes one tool call and returns the text handed back to the
// model. A tool that fails returns its error as content with isErr set rather
// than aborting the campaign: the model can read the failure and adapt, which
// is usually better than losing the trials already spent.
func (a *Agent) dispatchTool(ctx context.Context, name string, raw json.RawMessage) (content string, isErr bool) {
	switch name {
	case toolListWorkers:
		return a.callListWorkers(ctx)
	case toolClassifyWorkload:
		return a.callClassify(ctx, raw)
	case toolHistorical:
		return a.callHistorical(ctx, raw)
	case toolRunBenchmark:
		return a.callRunBenchmark(ctx, raw)
	case toolRecommend:
		return a.callRecommend(raw)
	default:
		// Unreachable unless the tool table and this switch drift apart.
		return fmt.Sprintf("unknown tool: %s", name), true
	}
}

func (a *Agent) callListWorkers(ctx context.Context) (string, bool) {
	a.emit(Event{Kind: EventLookup, Text: "checking registered workers"})

	workers, err := a.platform.ListWorkers(ctx)
	if err != nil {
		return fmt.Sprintf("could not list workers: %v", err), true
	}
	if len(workers) == 0 {
		return "No workers are registered. No benchmark can run until one registers.", false
	}
	return encodeJSON(workers), false
}

func (a *Agent) callClassify(ctx context.Context, raw json.RawMessage) (string, bool) {
	var in struct {
		StructuredOutput  bool    `json:"structured_output"`
		ToolCalls         bool    `json:"tool_calls"`
		SharedPrefixRatio float64 `json:"shared_prefix_ratio"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Sprintf("could not parse arguments: %v", err), true
	}

	a.emit(Event{Kind: EventLookup, Text: "classifying workload shape"})

	// The workload dimensions are the campaign's, not the model's to choose.
	result, err := a.platform.ClassifyWorkload(ctx, router.ClassificationInput{
		PromptTokens:      a.campaign.Workload.PromptTokens,
		OutputTokens:      a.campaign.Workload.OutputTokens,
		Concurrency:       a.campaign.Workload.Concurrency,
		StructuredOutput:  in.StructuredOutput,
		ToolCalls:         in.ToolCalls,
		SharedPrefixRatio: in.SharedPrefixRatio,
	})
	if err != nil {
		return fmt.Sprintf("could not classify workload: %v", err), true
	}
	return encodeJSON(result), false
}

func (a *Agent) callHistorical(ctx context.Context, raw json.RawMessage) (string, bool) {
	var in struct {
		Engine     string `json:"engine"`
		SinceHours int    `json:"since_hours"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Sprintf("could not parse arguments: %v", err), true
	}
	if in.SinceHours <= 0 {
		in.SinceHours = 168
	}

	a.emit(Event{Kind: EventLookup, Text: fmt.Sprintf("looking up past %s results", in.Engine)})

	since := a.spend.now().Add(-time.Duration(in.SinceHours) * time.Hour)
	results, err := a.platform.HistoricalResults(ctx, in.Engine, a.campaign.Model, since)
	if err != nil {
		return fmt.Sprintf("could not read historical results: %v", err), true
	}
	if len(results) == 0 {
		return fmt.Sprintf("No results for %s on %s in the last %d hours.",
			in.Engine, a.campaign.Model, in.SinceHours), false
	}
	return encodeJSON(results), false
}

func (a *Agent) callRunBenchmark(ctx context.Context, raw json.RawMessage) (string, bool) {
	var in struct {
		Engine     string `json:"engine"`
		Hypothesis string `json:"hypothesis"`
		engineConfigInput
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Sprintf("could not parse arguments: %v", err), true
	}

	if !a.allowedEngines[in.Engine] {
		return fmt.Sprintf("engine %q is not available in this campaign; choose one of: %s",
			in.Engine, strings.Join(a.campaign.Engines, ", ")), true
	}

	// Budget is checked before the trial starts, never after — a campaign
	// should not begin work it cannot afford to finish.
	if done, why := a.spend.exhausted(); done {
		a.emit(Event{Kind: EventWarn, Text: why})
		return fmt.Sprintf(
			"Trial refused: %s. Call %s now with the best configuration you measured.",
			why, toolRecommend), true
	}

	spec := TrialSpec{
		Model:        a.campaign.Model,
		Engine:       in.Engine,
		GPUProfile:   a.campaign.GPUProfile,
		Workload:     a.campaign.Workload,
		EngineConfig: in.engineConfigInput.toJobs(),
	}

	// Re-running an identical configuration buys nothing but costs a trial.
	if prior, ok := a.findTrial(spec); ok {
		a.trials = append(a.trials, Trial{
			Spec: spec, Result: prior.Result, Err: prior.Err,
			Hypothesis: in.Hypothesis, Reused: true,
		})
		return fmt.Sprintf(
			"This exact configuration was already measured in this campaign; reusing that result without spending a trial.\n%s",
			encodeJSON(prior.Result)), false
	}

	a.emit(Event{Kind: EventTrialStart, Text: in.Hypothesis, Trial: &spec})

	start := a.spend.now()
	result, err := a.platform.RunBenchmark(ctx, spec)
	elapsed := a.spend.now().Sub(start)
	a.spend.trials++

	trial := Trial{Spec: spec, Result: result, Duration: elapsed, Hypothesis: in.Hypothesis}
	if err != nil {
		trial.Err = err.Error()
		a.trials = append(a.trials, trial)
		a.emit(Event{Kind: EventTrialFailed, Text: err.Error(), Trial: &spec})
		return fmt.Sprintf("Benchmark failed: %v\n%d of %d trials remain.",
			err, a.spend.trialsLeft(), a.campaign.Budget.MaxTrials), true
	}

	a.trials = append(a.trials, trial)
	a.emit(Event{Kind: EventTrialDone, Trial: &spec, Result: &result})

	return fmt.Sprintf("%s\n\n%d of %d trials remain; %s of wall clock left.",
		encodeJSON(result), a.spend.trialsLeft(), a.campaign.Budget.MaxTrials,
		a.spend.timeLeft().Round(time.Second)), false
}

func (a *Agent) callRecommend(raw json.RawMessage) (string, bool) {
	var in struct {
		Engine     string `json:"engine"`
		Reasoning  string `json:"reasoning"`
		RunnerUp   string `json:"runner_up"`
		Confidence string `json:"confidence"`
		Caveats    string `json:"caveats"`
		engineConfigInput
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Sprintf("could not parse arguments: %v", err), true
	}

	rec := &Recommendation{
		Engine:       in.Engine,
		EngineConfig: in.engineConfigInput.toJobs(),
		Reasoning:    in.Reasoning,
		RunnerUp:     in.RunnerUp,
		Confidence:   in.Confidence,
		Caveats:      in.Caveats,
	}

	// Attach the measurement the recommendation refers to, so the report's
	// headline numbers are read off a trial rather than off the model's prose.
	if t, ok := a.findTrial(TrialSpec{
		Model: a.campaign.Model, Engine: in.Engine, GPUProfile: a.campaign.GPUProfile,
		Workload: a.campaign.Workload, EngineConfig: in.engineConfigInput.toJobs(),
	}); ok && t.OK() {
		r := t.Result
		rec.Measured = &r
	}

	a.recommendation = rec
	a.emit(Event{Kind: EventDone, Text: in.Reasoning})
	return "Recommendation recorded. The campaign is complete.", false
}

// findTrial returns an earlier trial of the same specification, if any.
func (a *Agent) findTrial(spec TrialSpec) (Trial, bool) {
	for _, t := range a.trials {
		if t.Spec == spec {
			return t, true
		}
	}
	return Trial{}, false
}

func encodeJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("could not encode result: %v", err)
	}
	return string(b)
}
