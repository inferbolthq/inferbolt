package workers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerRegistry_RegisterAndSelect(t *testing.T) {
	r := NewWorkerRegistry()
	r.Register(WorkerEntry{ID: "w1", URL: "http://127.0.0.1:9101", GPUType: "cpu"})

	entry, err := r.SelectWorker("cpu")
	require.NoError(t, err)
	assert.Equal(t, "w1", entry.ID)
	assert.Equal(t, StatusIdle, entry.Status)
}

func TestWorkerRegistry_SelectWorker_NoMatch(t *testing.T) {
	r := NewWorkerRegistry()
	r.Register(WorkerEntry{ID: "w1", URL: "http://127.0.0.1:9101", GPUType: "cpu"})

	_, err := r.SelectWorker("a100-80gb")
	assert.ErrorIs(t, err, ErrNoAvailableWorker)
}

func TestWorkerRegistry_MarkBusyThenIdle_ExcludesFromSelection(t *testing.T) {
	r := NewWorkerRegistry()
	r.Register(WorkerEntry{ID: "w1", URL: "http://127.0.0.1:9101", GPUType: "cpu"})
	r.MarkBusy("w1", "job-123")

	_, err := r.SelectWorker("cpu")
	assert.ErrorIs(t, err, ErrNoAvailableWorker, "busy worker must not be selectable")

	r.MarkIdle("w1")
	entry, err := r.SelectWorker("cpu")
	require.NoError(t, err)
	assert.Equal(t, "w1", entry.ID)
	assert.Empty(t, entry.CurrentJobID)
}

func TestWorkerRegistry_List_ReturnsSortedSnapshot(t *testing.T) {
	r := NewWorkerRegistry()
	r.Register(WorkerEntry{ID: "w2", URL: "http://127.0.0.1:9102", GPUType: "a100-80gb"})
	r.Register(WorkerEntry{ID: "w1", URL: "http://127.0.0.1:9101", GPUType: "cpu"})

	list := r.List()
	require.Len(t, list, 2)
	assert.Equal(t, "w1", list[0].ID)
	assert.Equal(t, "w2", list[1].ID)

	// Mutating the returned slice must not affect the registry's internal state.
	list[0].Status = StatusOffline
	fresh, err := r.SelectWorker("cpu")
	require.NoError(t, err, "snapshot mutation must not leak into registry state")
	assert.Equal(t, StatusIdle, fresh.Status)
}

func TestWorkerRegistry_HasIdle(t *testing.T) {
	r := NewWorkerRegistry()
	assert.False(t, r.HasIdle("cpu"), "empty registry has no idle workers")

	r.Register(WorkerEntry{ID: "w1", URL: "http://127.0.0.1:9101", GPUType: "cpu"})
	assert.True(t, r.HasIdle("cpu"))
	assert.False(t, r.HasIdle("a100-80gb"), "gpu type must match exactly")

	r.MarkBusy("w1", "job-123")
	assert.False(t, r.HasIdle("cpu"), "busy worker is not idle")
}

func TestWorkerRegistry_EvictStale(t *testing.T) {
	r := NewWorkerRegistry()
	r.Register(WorkerEntry{ID: "w1", URL: "http://127.0.0.1:9101", GPUType: "cpu"})

	// Simulate a worker that hasn't heartbeat in a while by backdating LastSeen directly.
	r.mu.Lock()
	r.workers["w1"].LastSeen = time.Now().Add(-time.Minute)
	r.mu.Unlock()

	r.EvictStale(30 * time.Second)
	assert.False(t, r.HasIdle("cpu"), "stale worker must be evicted to offline")

	r.Heartbeat("w1")
	assert.True(t, r.HasIdle("cpu"), "heartbeat must bring an offline worker back to idle")
}
