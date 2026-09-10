package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/inferbolthq/inferbolt/internal/jobs"
)

// DefaultModel plans the campaign. It does not run any benchmark and never
// sees a model under test — it only decides which configurations are worth
// measuring and reads the measurements back.
const DefaultModel = "claude-opus-5"

// maxResponseTokens is generous because a campaign turn may include a long
// hypothesis plus a full recommendation.
const maxResponseTokens = 16000

// ErrNoRecommendation is returned when a campaign ended without any usable
// measurement to recommend from.
var ErrNoRecommendation = errors.New("agent: campaign produced no usable measurement")

// Campaign is one optimization run. Model, GPUProfile and Workload are fixed
// for its whole duration: trials are only comparable against each other if the
// thing being measured is the configuration and nothing else.
type Campaign struct {
	Goal       string
	Model      string
	GPUProfile string
	Workload   jobs.WorkloadConfig
	Engines    []string
	Budget     Budget
}

// Recommendation is the campaign's conclusion.
type Recommendation struct {
	Engine       string            `json:"engine"`
	EngineConfig jobs.EngineConfig `json:"engine_config"`
	Reasoning    string            `json:"reasoning"`
	RunnerUp     string            `json:"runner_up,omitempty"`
	Confidence   string            `json:"confidence"`
	Caveats      string            `json:"caveats,omitempty"`

	// Measured is the trial this recommendation refers to, when one matches.
	// Report headline figures are read from here, never from the prose.
	Measured *jobs.Result `json:"measured,omitempty"`

	// Fallback marks a recommendation chosen mechanically from the trials
	// because the campaign ran out of budget or turns before the model
	// concluded. It is not the model's judgement and should be labelled as
	// such wherever it is shown.
	Fallback bool `json:"fallback,omitempty"`
}

// Report is everything a campaign produced.
type Report struct {
	Campaign       Campaign        `json:"campaign"`
	Recommendation *Recommendation `json:"recommendation"`
	Trials         []Trial         `json:"trials"`
	Usage          Usage           `json:"usage"`
	PlannerModel   string          `json:"planner_model"`
	Elapsed        time.Duration   `json:"elapsed"`
}

// messageCreator is the slice of the Anthropic client this package uses.
// Declaring it lets the campaign loop be tested against a scripted planner
// with no network and no API key.
type messageCreator interface {
	New(ctx context.Context, params anthropic.MessageNewParams, opts ...option.RequestOption) (*anthropic.Message, error)
}

// Agent runs one campaign. It is not safe for concurrent use.
type Agent struct {
	platform Platform
	client   messageCreator
	model    string
	campaign Campaign
	logger   *slog.Logger
	observer Observer

	allowedEngines map[string]bool
	trials         []Trial
	recommendation *Recommendation
	spend          *spend
}

// Option configures an Agent.
type Option func(*Agent)

// WithObserver streams campaign events as they happen.
func WithObserver(o Observer) Option { return func(a *Agent) { a.observer = o } }

// WithModel overrides the planning model.
func WithModel(m string) Option {
	return func(a *Agent) {
		if m != "" {
			a.model = m
		}
	}
}

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option {
	return func(a *Agent) {
		if l != nil {
			a.logger = l
		}
	}
}

// withPlanner injects a scripted planner in place of the Anthropic client.
func withPlanner(c messageCreator) Option { return func(a *Agent) { a.client = c } }

// withClock injects a clock for deterministic budget tests.
func withClock(now func() time.Time) Option {
	return func(a *Agent) { a.spend = newSpend(a.campaign.Budget, now) }
}

// New validates the campaign and builds an Agent. The Anthropic client resolves
// credentials the usual way (ANTHROPIC_API_KEY, or a configured CLI profile);
// no key is read or stored by this package.
func New(platform Platform, c Campaign, opts ...Option) (*Agent, error) {
	if platform == nil {
		return nil, errors.New("agent: platform is required")
	}
	if c.Goal == "" {
		return nil, errors.New("agent: campaign goal is required")
	}
	if c.Model == "" {
		return nil, errors.New("agent: campaign model is required")
	}
	if c.GPUProfile == "" {
		return nil, errors.New("agent: campaign gpu_profile is required")
	}
	if len(c.Engines) == 0 {
		return nil, errors.New("agent: at least one candidate engine is required")
	}
	if err := c.Budget.Validate(); err != nil {
		return nil, err
	}

	allowed := make(map[string]bool, len(c.Engines))
	for _, e := range c.Engines {
		allowed[e] = true
	}

	a := &Agent{
		platform:       platform,
		model:          DefaultModel,
		campaign:       c,
		logger:         slog.Default(),
		allowedEngines: allowed,
		spend:          newSpend(c.Budget, time.Now),
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.client == nil {
		client := anthropic.NewClient()
		a.client = &client.Messages
	}
	return a, nil
}

// Run executes the campaign, blocking until the agent submits a recommendation
// or the budget runs out. Trials run one at a time: they contend for the same
// GPU worker, so there is nothing to gain by overlapping them.
func (a *Agent) Run(ctx context.Context) (*Report, error) {
	// A campaign that overruns its wall clock is cancelled outright, including
	// any benchmark still in flight.
	ctx, cancel := context.WithTimeout(ctx, a.campaign.Budget.MaxDuration)
	defer cancel()

	system := []anthropic.TextBlockParam{{
		Text: systemPrompt(a.campaign),
		// Tools and system render ahead of messages, so one breakpoint here
		// caches the entire stable prefix for every later turn in the campaign.
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}}
	tools := toolDefinitions(a.campaign.Engines)

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(
			"Begin the campaign. Check what workers are available first, then plan and run your trials.")),
	}

	adaptive := anthropic.ThinkingConfigAdaptiveParam{
		Display: anthropic.ThinkingConfigAdaptiveDisplaySummarized,
	}

	for a.spend.turns < a.campaign.Budget.MaxTurns {
		a.spend.turns++

		resp, err := a.client.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(a.model),
			MaxTokens: maxResponseTokens,
			System:    system,
			Messages:  messages,
			Tools:     tools,
			Thinking:  anthropic.ThinkingConfigParamUnion{OfAdaptive: &adaptive},
		})
		if err != nil {
			return a.report(), fmt.Errorf("planner turn %d: %w", a.spend.turns, annotatePlannerError(err))
		}

		a.spend.usage.InputTokens += resp.Usage.InputTokens
		a.spend.usage.OutputTokens += resp.Usage.OutputTokens
		a.spend.usage.CacheReadTokens += resp.Usage.CacheReadInputTokens
		a.spend.usage.CacheCreationTokens += resp.Usage.CacheCreationInputTokens

		if resp.StopReason == anthropic.StopReasonRefusal {
			return a.report(), fmt.Errorf("planner declined the request (%s): %s",
				resp.StopDetails.Category, resp.StopDetails.Explanation)
		}

		// The assistant turn goes into history before its tool calls are run,
		// thinking blocks included — they have to come back unchanged.
		messages = append(messages, resp.ToParam())

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			switch v := block.AsAny().(type) {
			case anthropic.TextBlock:
				if v.Text != "" {
					a.emit(Event{Kind: EventPlan, Text: v.Text})
				}
			case anthropic.ThinkingBlock:
				if v.Thinking != "" {
					a.emit(Event{Kind: EventThinking, Text: v.Thinking})
				}
			case anthropic.ToolUseBlock:
				content, isErr := a.dispatchTool(ctx, v.Name, json.RawMessage(v.JSON.Input.Raw()))
				a.logger.Debug("agent tool call",
					"tool", v.Name, "turn", a.spend.turns, "error", isErr)
				toolResults = append(toolResults, anthropic.NewToolResultBlock(v.ID, content, isErr))
			}
		}

		if a.recommendation != nil {
			return a.report(), nil
		}
		if resp.StopReason != anthropic.StopReasonToolUse {
			// The planner stopped talking without concluding.
			break
		}
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	if err := a.concludeFromTrials(); err != nil {
		return a.report(), err
	}
	return a.report(), nil
}

// annotatePlannerError adds an actionable hint to the failures a user is most
// likely to hit, rather than surfacing a bare HTTP status.
func annotatePlannerError(err error) error {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.StatusCode {
	case 401, 403:
		return fmt.Errorf("%w — set ANTHROPIC_API_KEY, or sign in with the Anthropic CLI", err)
	case 429:
		return fmt.Errorf("%w — the planner is rate limited; retry, or lower --max-trials", err)
	default:
		return err
	}
}

// concludeFromTrials salvages a campaign that spent real GPU time but ended
// without the planner committing to an answer. The pick is mechanical and is
// labelled as such — the user is owed the measurements either way.
func (a *Agent) concludeFromTrials() error {
	best, ok := bestTrial(a.trials)
	if !ok {
		return ErrNoRecommendation
	}

	a.emit(Event{Kind: EventWarn,
		Text: "planner ended without a recommendation; reporting the best measured trial"})

	result := best.Result
	a.recommendation = &Recommendation{
		Engine:       best.Spec.Engine,
		EngineConfig: best.Spec.EngineConfig,
		Confidence:   "low",
		Fallback:     true,
		Reasoning: fmt.Sprintf(
			"Selected mechanically after the campaign ended without a conclusion: "+
				"lowest measured cost per million tokens ($%.4f at %.0f tok/s) among %d trials.",
			result.CostPerMTok, result.TokPerSec, len(a.trials)),
		Caveats:  "Not the planner's judgement. The campaign hit its budget or turn limit first.",
		Measured: &result,
	}
	return nil
}

// bestTrial picks the cheapest error-free trial. Cost per million tokens is
// throughput expressed in money, so this also picks the fastest at a fixed GPU
// price — the two only diverge across GPU profiles, and a campaign has one.
func bestTrial(trials []Trial) (Trial, bool) {
	var best Trial
	found := false
	for _, t := range trials {
		if !t.OK() || t.Result.ErrorRate > 0 || t.Result.CostPerMTok <= 0 {
			continue
		}
		if !found || t.Result.CostPerMTok < best.Result.CostPerMTok {
			best, found = t, true
		}
	}
	return best, found
}

func (a *Agent) report() *Report {
	a.spend.usage.Turns = a.spend.turns
	return &Report{
		Campaign:       a.campaign,
		Recommendation: a.recommendation,
		Trials:         a.trials,
		Usage:          a.spend.usage,
		PlannerModel:   a.model,
		Elapsed:        a.spend.elapsed(),
	}
}
