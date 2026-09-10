import asyncio
import hashlib
import random

from worker.engines.base import BaseEngine
from worker.models import EngineConfig, RawRequestResult

# Relative decode speedup per quantization. These are plausible orders of
# magnitude, not measurements — see the class docstring.
_QUANT_SPEEDUP: dict[str, float] = {
    "fp8": 1.35,
    "int8": 1.25,
    "int4": 1.70,
    "gptq": 1.50,
    "awq": 1.55,
}

# Fraction of full-precision weight memory retained per quantization.
_QUANT_MEMORY: dict[str, float] = {
    "fp8": 0.55,
    "int8": 0.55,
    "int4": 0.30,
    "gptq": 0.35,
    "awq": 0.35,
}

_BASE_WEIGHT_MB = 16_000
_BASE_TTFT_MS = 140.0
_BASE_ITL_MS = 20.0
_REFERENCE_BATCH = 256


class MockEngine(BaseEngine):
    """Fake engine for no-GPU smoke-testing the orchestration path end-to-end.

    Nothing here touches an inference runtime. The numbers are a *simulation*:
    they respond to the engine config in the direction real hardware would
    (quantization speeds up decode and shrinks weights, tensor parallelism
    splits work across GPUs with sublinear scaling, large batches trade
    time-to-first-token for throughput) so that the optimization agent and the
    config-sweep path have a gradient to follow without a GPU. They are not
    predictions of any real engine's performance and must never be presented
    as measurements.

    Timings are seeded from (model, config), so the same configuration scores
    the same on every run and a sweep over configs is reproducible.
    """

    def __init__(self) -> None:
        self._config = EngineConfig()
        self._rng = random.Random()
        self._ttft_ms = _BASE_TTFT_MS
        self._itl_ms = _BASE_ITL_MS

    def name(self) -> str:
        return "mock"

    def start(self, model: str, config: EngineConfig) -> None:
        self._config = config

        seed = hashlib.sha256(
            f"{model}|{config.model_dump_json()}".encode()
        ).digest()
        self._rng = random.Random(int.from_bytes(seed[:8], "big"))

        speedup = _QUANT_SPEEDUP.get(config.quantization or "", 1.0)
        # Sublinear tensor-parallel scaling: each extra GPU adds less than a
        # full GPU of throughput, and adds a little cross-device latency.
        tp = max(1, config.tensor_parallel)
        tp_speedup = 1.0 + 0.75 * (tp - 1)
        tp_overhead_ms = 4.0 * (tp - 1)

        batch_ratio = max(1, config.max_batch_size) / _REFERENCE_BATCH

        self._itl_ms = _BASE_ITL_MS / (speedup * tp_speedup)
        # Bigger batches queue longer before the first token comes back.
        self._ttft_ms = (_BASE_TTFT_MS * (0.85 + 0.35 * batch_ratio)) / tp_speedup + tp_overhead_ms

    def teardown(self) -> None:
        pass

    def get_gpu_memory_mb(self) -> int:
        weights = _BASE_WEIGHT_MB * _QUANT_MEMORY.get(self._config.quantization or "", 1.0)
        per_device = weights / max(1, self._config.tensor_parallel)
        kv_cache = 40 * max(1, self._config.max_batch_size)
        return int(per_device + kv_cache)

    def get_kv_cache_hit_rate(self) -> float:
        return round(self._rng.uniform(0.4, 0.95), 3)

    async def _send_request(self, prompt: str, max_tokens: int) -> RawRequestResult:
        await asyncio.sleep(0.05)  # simulate network + inference latency
        ttft_ms = round(self._ttft_ms * self._rng.uniform(0.9, 1.1), 2)
        itl_ms = round(self._itl_ms * self._rng.uniform(0.9, 1.1), 2)
        return RawRequestResult(
            ttft_ms=ttft_ms,
            itl_ms=itl_ms,
            total_ms=ttft_ms + itl_ms * max_tokens,
            output_tokens=max_tokens,
        )
