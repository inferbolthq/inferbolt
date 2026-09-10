package agent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The campaign API is snake_case throughout, and max_duration is written the
// same way it is read: as a duration string, not a nanosecond count.
func TestBudget_WireFormat(t *testing.T) {
	b, err := json.Marshal(Budget{MaxTrials: 6, MaxDuration: 2 * time.Hour, MaxTurns: 40})
	require.NoError(t, err)

	assert.JSONEq(t, `{"max_trials":6,"max_duration":"2h0m0s","max_turns":40}`, string(b))
}

func TestBudget_RoundTrips(t *testing.T) {
	original := Budget{MaxTrials: 3, MaxDuration: 45 * time.Minute, MaxTurns: 12}

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var back Budget
	require.NoError(t, json.Unmarshal(raw, &back))
	assert.Equal(t, original, back)
}

func TestBudget_RejectsAnUnparseableDuration(t *testing.T) {
	var b Budget
	err := json.Unmarshal([]byte(`{"max_trials":1,"max_duration":"soon","max_turns":1}`), &b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_duration")
}

func TestCampaign_WireFormatIsSnakeCase(t *testing.T) {
	raw, err := json.Marshal(Campaign{Goal: "g", Model: "m", GPUProfile: "cpu"})
	require.NoError(t, err)

	for _, want := range []string{`"goal"`, `"model"`, `"gpu_profile"`, `"workload"`, `"engines"`, `"budget"`} {
		assert.Contains(t, string(raw), want)
	}
	assert.NotContains(t, string(raw), `"GPUProfile"`)
}
