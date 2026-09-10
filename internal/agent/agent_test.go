package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/inferbolthq/inferbolt/internal/jobs"
	"github.com/inferbolthq/inferbolt/internal/router"
)

// ── scripted planner ──────────────────────────────────────────────────────────

// scriptedPlanner replays canned API responses in order. Responses are written
// as raw JSON and unmarshalled into anthropic.Message so that the union content
// blocks and their raw-input metadata are populated exactly as they would be by
// a real response — the loop reads tool arguments back out via JSON.Input.Raw().
type scriptedPlanner struct {
	responses []string
	calls     int
	seen      []anthropic.MessageNewParams
}

func (p *scriptedPlanner) New(_ context.Context, params anthropic.MessageNewParams, _ ...option.RequestOption) (*anthropic.Message, error) {
	p.seen = append(p.seen, params)
	if p.calls >= len(p.responses) {
		return nil, errors.New("scripted planner ran out of responses")
	}
	body := p.responses[p.calls]
	p.calls++

	var m anthropic.Message
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return nil, fmt.Errorf("bad scripted response: %w", err)
	}
	return &m, nil
}

func msg(stopReason string, blocks ...string) string {
	return fmt.Sprintf(`{
		"id": "msg_test", "type": "message", "role": "assistant",
		"model": "claude-opus-5",
		"content": [%s],
		"stop_reason": %q,
		"usage": {"input_tokens": 1000, "output_tokens": 200, "cache_read_input_tokens": 500, "cache_creation_input_tokens": 0}
	}`, strings.Join(blocks, ","), stopReason)
}

func textBlock(s string) string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf(`{"type":"text","text":%s}`, b)
}

func toolBlock(id, name string, input map[string]any) string {
	b, _ := json.Marshal(input)
	return fmt.Sprintf(`{"type":"tool_use","id":%q,"name":%q,"input":%s}`, id, name, b)
}

func benchmarkCall(id, engine, quant string, extra map[string]any) string {
	in := map[string]any{"engine": engine, "hypothesis": "probing " + quant}
	if quant != "" {
		in["quantization"] = quant
	}
	for k, v := range extra {
		in[k] = v
	}
	return toolBlock(id, toolRunBenchmark, in)
}

// ── fake platform ─────────────────────────────────────────────────────────────

type fakePlatform struct {
	workers    []Worker
	historical []jobs.Result
	runs       []TrialSpec
	runErr     error
	resultFor  func(TrialSpec) jobs.Result
}

func (f *fakePlatform) ListWorkers(context.Context) ([]Worker, error) {
	return f.workers, nil
}

func (f *fakePlatform) ClassifyWorkload(_ context.Context, in router.ClassificationInput) (router.ClassificationResult, error) {
	return router.Classify(in), nil
}

func (f *fakePlatform) HistoricalResults(context.Context, string, string, time.Time) ([]jobs.Result, error) {
	return f.historical, nil
}

func (f *fakePlatform) RunBenchmark(_ context.Context, spec TrialSpec) (jobs.Result, error) {
	f.runs = append(f.runs, spec)
	if f.runErr != nil {
		return jobs.Result{}, f.runErr
	}
	if f.resultFor != nil {
		return f.resultFor(spec), nil
	}
	return jobs.Result{
		Engine: spec.Engine, Model: spec.Model,
		TokPerSec: 3000, CostPerMTok: 0.30, TTFTP99Ms: 150,
	}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func testCampaign() Campaign {
	return Campaign{
		Goal:       "cheapest engine under 200ms p99 TTFT",
		Model:      "meta-llama/Llama-3.1-8B",
		GPUProfile: "a100-80gb",
		Workload:   jobs.WorkloadConfig{Concurrency: 32, PromptTokens: 512, OutputTokens: 256, NumRequests: 200},
		Engines:    []string{"vllm", "sglang"},
		Budget:     Budget{MaxTrials: 3, MaxDuration: time.Hour, MaxTurns: 10},
	}
}

func newTestAgent(t *testing.T, p Platform, responses []string, opts ...Option) (*Agent, *scriptedPlanner) {
	t.Helper()
	planner := &scriptedPlanner{responses: responses}
	opts = append([]Option{withPlanner(planner)}, opts...)
	a, err := New(p, testCampaign(), opts...)
	require.NoError(t, err)
	return a, planner
}

// ── construction ──────────────────────────────────────────────────────────────

func TestNew_ValidatesCampaign(t *testing.T) {
	valid := testCampaign()

	tests := []struct {
		name    string
		mutate  func(*Campaign)
		wantErr string
	}{
		{"missing goal", func(c *Campaign) { c.Goal = "" }, "goal is required"},
		{"missing model", func(c *Campaign) { c.Model = "" }, "model is required"},
		{"missing gpu profile", func(c *Campaign) { c.GPUProfile = "" }, "gpu_profile is required"},
		{"no engines", func(c *Campaign) { c.Engines = nil }, "at least one candidate engine"},
		{"zero trials", func(c *Campaign) { c.Budget.MaxTrials = 0 }, "max_trials"},
		{"zero duration", func(c *Campaign) { c.Budget.MaxDuration = 0 }, "max_duration"},
		{"zero turns", func(c *Campaign) { c.Budget.MaxTurns = 0 }, "max_turns"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid
			tt.mutate(&c)
			_, err := New(&fakePlatform{}, c, withPlanner(&scriptedPlanner{}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNew_RequiresPlatform(t *testing.T) {
	_, err := New(nil, testCampaign())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform is required")
}

// ── the campaign loop ─────────────────────────────────────────────────────────

func TestRun_MeasuresThenRecommends(t *testing.T) {
	p := &fakePlatform{workers: []Worker{{ID: "w1", GPUType: "a100-80gb", Status: "idle"}}}

	a, planner := newTestAgent(t, p, []string{
		msg("tool_use", textBlock("Checking hardware first."), toolBlock("t1", toolListWorkers, map[string]any{})),
		msg("tool_use", benchmarkCall("t2", "vllm", "fp8", nil)),
		msg("tool_use", toolBlock("t3", toolRecommend, map[string]any{
			"engine":       "vllm",
			"quantization": "fp8",
			"reasoning":    "fp8 hit 3000 tok/s at $0.30/Mtok within the TTFT ceiling",
			"confidence":   "high",
			"runner_up":    "sglang fp8",
		})),
	})

	var events []Event
	a.observer = func(e Event) { events = append(events, e) }

	report, err := a.Run(context.Background())
	require.NoError(t, err)

	require.NotNil(t, report.Recommendation)
	assert.Equal(t, "vllm", report.Recommendation.Engine)
	assert.Equal(t, "fp8", report.Recommendation.EngineConfig.Quantization)
	assert.False(t, report.Recommendation.Fallback)

	// The headline numbers must come off a real trial, not the model's prose.
	require.NotNil(t, report.Recommendation.Measured)
	assert.Equal(t, 3000.0, report.Recommendation.Measured.TokPerSec)

	require.Len(t, p.runs, 1)
	assert.Equal(t, "meta-llama/Llama-3.1-8B", p.runs[0].Model)
	assert.Equal(t, "a100-80gb", p.runs[0].GPUProfile)
	assert.Equal(t, 32, p.runs[0].Workload.Concurrency)
	assert.Equal(t, "fp8", p.runs[0].EngineConfig.Quantization)

	assert.Equal(t, 3, planner.calls)
	assert.Equal(t, int64(3000), report.Usage.InputTokens)
	assert.Equal(t, int64(600), report.Usage.OutputTokens)
	assert.Equal(t, 3, report.Usage.Turns)

	assert.Contains(t, kinds(events), EventTrialStart)
	assert.Contains(t, kinds(events), EventTrialDone)
	assert.Contains(t, kinds(events), EventDone)
}

// The campaign fixes the model, GPU profile and workload; the planner may only
// vary engine and configuration, so trials stay comparable.
func TestRun_PlannerCannotChangeTheWorkloadOrModel(t *testing.T) {
	p := &fakePlatform{}
	a, _ := newTestAgent(t, p, []string{
		msg("tool_use", toolBlock("t1", toolRunBenchmark, map[string]any{
			"engine":      "vllm",
			"hypothesis":  "trying to smuggle in a different model and workload",
			"model":       "some-other-model",
			"gpu_profile": "h100",
			"workload":    map[string]any{"concurrency": 1},
		})),
		msg("end_turn", textBlock("done")),
	})

	_, err := a.Run(context.Background())
	require.NoError(t, err)

	require.Len(t, p.runs, 1)
	assert.Equal(t, "meta-llama/Llama-3.1-8B", p.runs[0].Model)
	assert.Equal(t, "a100-80gb", p.runs[0].GPUProfile)
	assert.Equal(t, 32, p.runs[0].Workload.Concurrency)
}

func TestRun_RefusesEngineOutsideTheCampaign(t *testing.T) {
	p := &fakePlatform{}
	a, _ := newTestAgent(t, p, []string{
		msg("tool_use", benchmarkCall("t1", "tensorrt", "", nil)),
		msg("end_turn", textBlock("understood")),
	})

	// Nothing was measured, so there is correctly nothing to recommend.
	_, err := a.Run(context.Background())
	require.ErrorIs(t, err, ErrNoRecommendation)
	assert.Empty(t, p.runs, "an engine outside the campaign must never reach the platform")
}

func TestRun_StopsSubmittingTrialsOnceTheBudgetIsSpent(t *testing.T) {
	p := &fakePlatform{}

	// Five benchmark requests against a three-trial budget.
	responses := make([]string, 0, 6)
	for i := 0; i < 5; i++ {
		responses = append(responses, msg("tool_use",
			benchmarkCall(fmt.Sprintf("t%d", i), "vllm", "", map[string]any{"max_batch_size": 64 * (i + 1)})))
	}
	responses = append(responses, msg("tool_use", toolBlock("tf", toolRecommend, map[string]any{
		"engine": "vllm", "reasoning": "out of budget", "confidence": "low",
	})))

	a, _ := newTestAgent(t, p, responses)

	var warned bool
	a.observer = func(e Event) {
		if e.Kind == EventWarn {
			warned = true
		}
	}

	report, err := a.Run(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.runs, 3, "the budget is a hard cap on benchmarks actually submitted")
	assert.True(t, warned, "budget exhaustion should surface as a warning event")
	require.NotNil(t, report.Recommendation)
}

func TestRun_ReusesAnIdenticalConfigWithoutSpendingATrial(t *testing.T) {
	p := &fakePlatform{}
	a, _ := newTestAgent(t, p, []string{
		msg("tool_use", benchmarkCall("t1", "vllm", "fp8", nil)),
		msg("tool_use", benchmarkCall("t2", "vllm", "fp8", nil)),
		msg("tool_use", toolBlock("t3", toolRecommend, map[string]any{
			"engine": "vllm", "quantization": "fp8", "reasoning": "measured", "confidence": "medium",
		})),
	})

	report, err := a.Run(context.Background())
	require.NoError(t, err)

	assert.Len(t, p.runs, 1, "the same configuration must not be benchmarked twice")
	require.Len(t, report.Trials, 2)
	assert.False(t, report.Trials[0].Reused)
	assert.True(t, report.Trials[1].Reused)
	assert.Equal(t, 1, a.spend.trials, "a reused trial must not consume budget")
}

func TestRun_ABenchmarkFailureIsReportedToThePlannerNotFatal(t *testing.T) {
	p := &fakePlatform{runErr: errors.New("no available worker for gpu_profile=a100-80gb")}
	a, _ := newTestAgent(t, p, []string{
		msg("tool_use", benchmarkCall("t1", "vllm", "int4", nil)),
		msg("tool_use", toolBlock("t2", toolRecommend, map[string]any{
			"engine": "vllm", "reasoning": "nothing ran", "confidence": "low",
		})),
	})

	report, err := a.Run(context.Background())
	require.NoError(t, err)

	require.Len(t, report.Trials, 1)
	assert.Contains(t, report.Trials[0].Err, "no available worker")
	assert.False(t, report.Trials[0].OK())
	require.NotNil(t, report.Recommendation)
	assert.Nil(t, report.Recommendation.Measured, "a failed trial must not be cited as a measurement")
}

// A campaign that spent GPU time still owes the user an answer.
func TestRun_FallsBackToTheBestTrialWhenThePlannerNeverConcludes(t *testing.T) {
	p := &fakePlatform{resultFor: func(spec TrialSpec) jobs.Result {
		cost := map[string]float64{"": 0.40, "fp8": 0.28, "int4": 0.19}[spec.EngineConfig.Quantization]
		return jobs.Result{
			Engine: spec.Engine, Model: spec.Model,
			TokPerSec: 1 / cost * 1000, CostPerMTok: cost,
		}
	}}

	a, _ := newTestAgent(t, p, []string{
		msg("tool_use", benchmarkCall("t1", "vllm", "fp8", nil)),
		msg("tool_use", benchmarkCall("t2", "vllm", "int4", nil)),
		msg("end_turn", textBlock("I think that's enough data.")),
	})

	report, err := a.Run(context.Background())
	require.NoError(t, err)

	require.NotNil(t, report.Recommendation)
	assert.True(t, report.Recommendation.Fallback)
	assert.Equal(t, "low", report.Recommendation.Confidence)
	assert.Equal(t, "int4", report.Recommendation.EngineConfig.Quantization, "cheapest measured trial wins")
	assert.Contains(t, report.Recommendation.Caveats, "Not the planner's judgement")
}

func TestRun_NoMeasurementsMeansNoRecommendation(t *testing.T) {
	a, _ := newTestAgent(t, &fakePlatform{}, []string{
		msg("end_turn", textBlock("I decline to benchmark anything.")),
	})

	report, err := a.Run(context.Background())
	require.ErrorIs(t, err, ErrNoRecommendation)
	assert.Nil(t, report.Recommendation)
	assert.Empty(t, report.Trials)
}

func TestRun_SurfacesARefusal(t *testing.T) {
	a, _ := newTestAgent(t, &fakePlatform{}, []string{`{
		"id":"msg_r","type":"message","role":"assistant","model":"claude-opus-5",
		"content":[],"stop_reason":"refusal",
		"stop_details":{"type":"refusal","category":"cyber","explanation":"declined"},
		"usage":{"input_tokens":10,"output_tokens":0}
	}`})

	_, err := a.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "declined")
}

func TestRun_ReportsPlannerErrorsWithThePartialCampaign(t *testing.T) {
	p := &fakePlatform{}
	a, _ := newTestAgent(t, p, []string{
		msg("tool_use", benchmarkCall("t1", "vllm", "fp8", nil)),
		// script exhausted — the next turn errors
	})

	report, err := a.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "planner turn 2")
	require.NotNil(t, report)
	assert.Len(t, report.Trials, 1, "work already done is still reported")
}

func TestRun_HaltsAtTheTurnLimit(t *testing.T) {
	// A planner that only ever emits text never concludes; MaxTurns must stop it.
	responses := make([]string, 20)
	for i := range responses {
		responses[i] = msg("tool_use", toolBlock(fmt.Sprintf("t%d", i), toolListWorkers, map[string]any{}))
	}
	a, planner := newTestAgent(t, &fakePlatform{}, responses)

	_, err := a.Run(context.Background())
	require.ErrorIs(t, err, ErrNoRecommendation)
	assert.Equal(t, testCampaign().Budget.MaxTurns, planner.calls)
}

// ── prompt and request shape ──────────────────────────────────────────────────

func TestRun_RequestIsShapedForCaching(t *testing.T) {
	a, planner := newTestAgent(t, &fakePlatform{}, []string{
		msg("tool_use", benchmarkCall("t1", "vllm", "fp8", nil)),
		msg("tool_use", toolBlock("t2", toolRecommend, map[string]any{
			"engine": "vllm", "reasoning": "r", "confidence": "high",
		})),
	})
	_, err := a.Run(context.Background())
	require.NoError(t, err)
	require.Len(t, planner.seen, 2)

	first, second := planner.seen[0], planner.seen[1]

	require.Len(t, first.System, 1)
	// Assert on what actually goes on the wire rather than on the zero value of
	// a field whose default marshals identically when unset.
	wire, err := json.Marshal(first.System[0])
	require.NoError(t, err)
	assert.Contains(t, string(wire), `"cache_control"`, "the stable prefix should carry a cache breakpoint")

	// The prefix must be byte-identical across turns or nothing caches.
	assert.Equal(t, first.System[0].Text, second.System[0].Text)
	assert.Len(t, second.Tools, len(first.Tools))

	assert.Equal(t, DefaultModel, string(first.Model))
	assert.NotNil(t, first.Thinking.OfAdaptive, "adaptive thinking should be enabled")

	// Campaign facts belong in the prompt; the planner must not have to guess.
	sys := first.System[0].Text
	for _, want := range []string{"meta-llama/Llama-3.1-8B", "a100-80gb", "cheapest engine", "vllm, sglang"} {
		assert.Contains(t, sys, want)
	}
}

func TestSystemPrompt_HasNoPerTurnVariance(t *testing.T) {
	c := testCampaign()
	assert.Equal(t, systemPrompt(c), systemPrompt(c),
		"a prompt that varies between identical campaigns would defeat prompt caching")
}

// ── budget and selection ──────────────────────────────────────────────────────

func TestSpend_ExhaustsOnTime(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	s := newSpend(Budget{MaxTrials: 10, MaxDuration: time.Minute, MaxTurns: 5}, clock)

	done, _ := s.exhausted()
	assert.False(t, done)

	now = now.Add(2 * time.Minute)
	done, why := s.exhausted()
	assert.True(t, done)
	assert.Contains(t, why, "time budget exhausted")
}

func TestBestTrial_SkipsFailedAndErroringRuns(t *testing.T) {
	trials := []Trial{
		{Err: "dispatch failed"},
		{Result: jobs.Result{TokPerSec: 5000, CostPerMTok: 0.10, ErrorRate: 0.02}},
		{Result: jobs.Result{TokPerSec: 3000, CostPerMTok: 0.30}},
		{Result: jobs.Result{TokPerSec: 4000, CostPerMTok: 0.20}},
	}

	best, ok := bestTrial(trials)
	require.True(t, ok)
	assert.Equal(t, 0.20, best.Result.CostPerMTok,
		"a cheaper run that was dropping requests is not the winner")

	_, ok = bestTrial([]Trial{{Err: "boom"}})
	assert.False(t, ok)
}

func TestUsage_EstimateUSD(t *testing.T) {
	u := Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000}

	cost, known := u.EstimateUSD("claude-opus-5")
	require.True(t, known)
	assert.InDelta(t, 30.0, cost, 0.001)

	_, known = u.EstimateUSD("some-unreleased-model")
	assert.False(t, known, "an unknown model must report no estimate rather than a wrong one")
}

func kinds(events []Event) []EventKind {
	out := make([]EventKind, len(events))
	for i, e := range events {
		out[i] = e.Kind
	}
	return out
}
