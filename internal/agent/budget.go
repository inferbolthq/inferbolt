package agent

import (
	"encoding/json"
	"fmt"
	"time"
)

// Budget bounds what one campaign may spend. A benchmark trial occupies a GPU
// worker for minutes at a time and an unsupervised planner will happily keep
// exploring, so every campaign runs against hard caps rather than trusting the
// model to stop on its own.
type Budget struct {
	// MaxTrials caps benchmark jobs submitted. Reused trials do not count.
	MaxTrials int

	// MaxDuration caps wall-clock time for the whole campaign.
	MaxDuration time.Duration

	// MaxTurns caps model round-trips, so a model that stops calling tools
	// without concluding cannot spin. Not a cost lever — MaxTrials is.
	MaxTurns int
}

// budgetJSON is the wire form. MaxDuration crosses as a duration string
// ("2h0m0s") rather than the nanosecond integer a time.Duration marshals to by
// default: it is the same spelling POST /v1/campaigns accepts for max_duration,
// so the field reads and writes identically, and a UI can show it as-is.
type budgetJSON struct {
	MaxTrials   int    `json:"max_trials"`
	MaxDuration string `json:"max_duration"`
	MaxTurns    int    `json:"max_turns"`
}

func (b Budget) MarshalJSON() ([]byte, error) {
	return json.Marshal(budgetJSON{
		MaxTrials:   b.MaxTrials,
		MaxDuration: b.MaxDuration.String(),
		MaxTurns:    b.MaxTurns,
	})
}

func (b *Budget) UnmarshalJSON(data []byte) error {
	var raw budgetJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.MaxTrials = raw.MaxTrials
	b.MaxTurns = raw.MaxTurns

	if raw.MaxDuration == "" {
		b.MaxDuration = 0
		return nil
	}
	d, err := time.ParseDuration(raw.MaxDuration)
	if err != nil {
		return fmt.Errorf("budget: max_duration %q is not a duration: %w", raw.MaxDuration, err)
	}
	b.MaxDuration = d
	return nil
}

// DefaultBudget is deliberately small. Six trials against real hardware is
// already tens of minutes of GPU time.
func DefaultBudget() Budget {
	return Budget{MaxTrials: 6, MaxDuration: 2 * time.Hour, MaxTurns: 40}
}

// Validate rejects a budget that would either do nothing or run unbounded.
func (b Budget) Validate() error {
	switch {
	case b.MaxTrials < 1:
		return fmt.Errorf("budget: max_trials must be at least 1, got %d", b.MaxTrials)
	case b.MaxDuration <= 0:
		return fmt.Errorf("budget: max_duration must be positive, got %s", b.MaxDuration)
	case b.MaxTurns < 1:
		return fmt.Errorf("budget: max_turns must be at least 1, got %d", b.MaxTurns)
	}
	return nil
}

// spend tracks consumption against a Budget for one campaign.
type spend struct {
	budget  Budget
	started time.Time
	trials  int
	turns   int
	usage   Usage
	nowFunc func() time.Time
}

func newSpend(b Budget, now func() time.Time) *spend {
	if now == nil {
		now = time.Now
	}
	return &spend{budget: b, started: now(), nowFunc: now}
}

func (s *spend) now() time.Time { return s.nowFunc() }

func (s *spend) elapsed() time.Duration { return s.now().Sub(s.started) }

func (s *spend) trialsLeft() int { return s.budget.MaxTrials - s.trials }

func (s *spend) timeLeft() time.Duration { return s.budget.MaxDuration - s.elapsed() }

// exhausted reports whether no further trial may start, and why. It is checked
// before a trial rather than after, so the campaign never starts work it cannot
// afford to finish.
func (s *spend) exhausted() (bool, string) {
	if s.trials >= s.budget.MaxTrials {
		return true, fmt.Sprintf("trial budget exhausted (%d of %d used)", s.trials, s.budget.MaxTrials)
	}
	if s.elapsed() >= s.budget.MaxDuration {
		return true, fmt.Sprintf("time budget exhausted (%s of %s elapsed)",
			s.elapsed().Round(time.Second), s.budget.MaxDuration)
	}
	return false, ""
}

// Usage is the model-side cost of a campaign. GPU time is reported separately
// as trials and wall-clock; this is the planning overhead only.
type Usage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	Turns               int   `json:"turns"`
}

// modelPricing is USD per million tokens, keyed by model id. Only models whose
// price is known are listed: an unknown model reports no cost estimate rather
// than a confidently wrong one.
var modelPricing = map[string]struct{ input, output float64 }{
	"claude-opus-5":    {5.00, 25.00},
	"claude-sonnet-5":  {2.00, 10.00},
	"claude-haiku-4-5": {1.00, 5.00},
}

// EstimateUSD returns the approximate planning cost and whether the model's
// price is known. Cache reads bill at roughly a tenth of the input rate and
// cache writes at roughly 1.25x, which is what the multipliers below encode.
func (u Usage) EstimateUSD(model string) (float64, bool) {
	p, ok := modelPricing[model]
	if !ok {
		return 0, false
	}
	const perMillion = 1_000_000.0
	cost := float64(u.InputTokens)/perMillion*p.input +
		float64(u.OutputTokens)/perMillion*p.output +
		float64(u.CacheReadTokens)/perMillion*p.input*0.1 +
		float64(u.CacheCreationTokens)/perMillion*p.input*1.25
	return cost, true
}
