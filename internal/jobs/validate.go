package jobs

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
)

// ValidEngines is the set of engine adapters the platform can dispatch to. It
// mirrors worker/engines/__init__.py's ENGINES registry; the two are maintained
// by hand and have drifted before, so check both when adding an adapter.
var ValidEngines = map[string]bool{
	"vllm": true, "sglang": true, "tensorrt": true,
	"llamacpp": true, "ollama": true, "mock": true,
}

// ValidQuantizations is closed because the value is forwarded verbatim as an
// engine CLI argument (vllm --quantization, sglang --quantization).
var ValidQuantizations = map[string]bool{
	"fp8": true, "int8": true, "int4": true, "gptq": true, "awq": true,
}

// ValidateEngines rejects any engine the platform cannot dispatch to.
func ValidateEngines(engines []string) error {
	for _, e := range engines {
		if !ValidEngines[e] {
			return errors.New("unknown engine: " + e)
		}
	}
	return nil
}

// ValidateWorkload bounds the shape of a benchmark run. These are rejections,
// not clamps: a caller that asks for something unrunnable should hear about it
// rather than silently get a different benchmark than it requested. Zero
// concurrency is the sharpest edge — it wedges the worker on an
// asyncio.Semaphore(0) until the orchestrator's 45-minute poll timeout fires.
//
// Lives in the domain package because there are now two entry points into the
// job pipeline — the gateway handler and the campaign runner — and they must
// agree on what is dispatchable.
func ValidateWorkload(w WorkloadConfig) error {
	switch {
	case w.Concurrency < 1 || w.Concurrency > 1024:
		return errors.New("workload.concurrency must be between 1 and 1024")
	case w.PromptTokens < 1 || w.PromptTokens > 1_000_000:
		return errors.New("workload.prompt_tokens must be between 1 and 1000000")
	case w.OutputTokens < 1 || w.OutputTokens > 1_000_000:
		return errors.New("workload.output_tokens must be between 1 and 1000000")
	case w.NumRequests < 1 || w.NumRequests > 100_000:
		return errors.New("workload.num_requests must be between 1 and 100000")
	}
	return nil
}

// ValidateEngineConfig bounds the engine tuning knobs. A zero value means
// "unset" and is left to the worker's own default, so only non-zero fields are
// range-checked.
func ValidateEngineConfig(c EngineConfig) error {
	switch {
	case c.Quantization != "" && !ValidQuantizations[c.Quantization]:
		return errors.New("unknown quantization: " + c.Quantization)
	case c.TensorParallel < 0 || c.TensorParallel > 8:
		return errors.New("engine_config.tensor_parallel must be between 1 and 8 (0 = engine default)")
	case c.MaxBatchSize < 0 || c.MaxBatchSize > 4096:
		return errors.New("engine_config.max_batch_size must be between 1 and 4096 (0 = engine default)")
	case c.MaxModelLen < 0 || c.MaxModelLen > 1_048_576:
		return errors.New("engine_config.max_model_len must be between 1 and 1048576 (0 = engine default)")
	case c.GPUMemoryUtilization < 0 || c.GPUMemoryUtilization > 1:
		return errors.New("engine_config.gpu_memory_utilization must be between 0 and 1 (0 = engine default)")
	}
	return nil
}

// NewID returns a random UUIDv4 string, used for job and campaign identifiers.
func NewID() string {
	b := make([]byte, 16)
	_, _ = cryptorand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:])
}
