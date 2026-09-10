"""Tests for the mock engine — the no-GPU path the docs and `inferbolt run` rely on.

Regression coverage for two shipped bugs: MockEngine implemented an interface
that no longer existed, and it was missing from the ENGINES registry, so
`--engines mock` failed before it ran.
"""

from worker.engines import ENGINES, get_engine
from worker.engines.base import BaseEngine
from worker.engines.mock_engine import MockEngine
from worker.models import EngineConfig, WorkloadConfig

_MODEL = "meta-llama/Llama-3.1-8B"
_WORKLOAD = WorkloadConfig(concurrency=4, prompt_tokens=128, output_tokens=32, num_requests=8)


def _run(config: EngineConfig) -> MockEngine:
    engine = MockEngine()
    engine.start(_MODEL, config)
    return engine


def test_mock_is_registered():
    assert "mock" in ENGINES
    assert isinstance(get_engine("mock"), MockEngine)


def test_mock_satisfies_the_base_engine_contract():
    engine = MockEngine()
    assert isinstance(engine, BaseEngine)
    assert engine.name() == "mock"


async def test_benchmark_returns_one_result_per_request():
    engine = _run(EngineConfig())
    raw = await engine.benchmark(_MODEL, _WORKLOAD)
    assert len(raw) == _WORKLOAD.num_requests
    assert all(r.error is None for r in raw)


async def test_compute_result_carries_the_engine_config():
    # The agent attributes measurements to the config that produced them, so
    # the config has to survive into the result row.
    config = EngineConfig(quantization="fp8", tensor_parallel=2)
    engine = _run(config)
    raw = await engine.benchmark(_MODEL, _WORKLOAD)
    result = engine.compute_result("job-1", "run-1", _MODEL, "a100-80gb", raw, config)

    assert result.engine == "mock"
    assert result.config["quantization"] == "fp8"
    assert result.config["tensor_parallel"] == 2
    assert result.tok_per_s > 0


def test_quantization_speeds_up_decode_and_shrinks_weights():
    baseline = _run(EngineConfig())
    int4 = _run(EngineConfig(quantization="int4"))

    assert int4._itl_ms < baseline._itl_ms
    assert int4.get_gpu_memory_mb() < baseline.get_gpu_memory_mb()


def test_tensor_parallelism_scales_sublinearly():
    tp1 = _run(EngineConfig(tensor_parallel=1))
    tp2 = _run(EngineConfig(tensor_parallel=2))

    assert tp2._itl_ms < tp1._itl_ms
    # Two GPUs must not look like a free 2x, or a sweep would always max out TP.
    assert tp2._itl_ms > tp1._itl_ms / 2


def test_larger_batches_trade_ttft_for_throughput():
    small = _run(EngineConfig(max_batch_size=64))
    large = _run(EngineConfig(max_batch_size=1024))

    assert large._ttft_ms > small._ttft_ms


async def test_same_config_scores_the_same_twice():
    # A sweep is only meaningful if a repeated configuration is reproducible.
    config = EngineConfig(quantization="fp8", tensor_parallel=2, max_batch_size=128)

    first = await _run(config).benchmark(_MODEL, _WORKLOAD)
    second = await _run(config).benchmark(_MODEL, _WORKLOAD)

    assert [r.ttft_ms for r in first] == [r.ttft_ms for r in second]
    assert [r.itl_ms for r in first] == [r.itl_ms for r in second]
