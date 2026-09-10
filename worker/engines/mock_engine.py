import asyncio
import random

from worker.engines.base import BaseEngine
from worker.models import EngineConfig, RawRequestResult


class MockEngine(BaseEngine):
    """Fake engine for no-GPU smoke-testing the orchestration path end-to-end."""

    def name(self) -> str:
        return "mock"

    def start(self, model: str, config: EngineConfig) -> None:
        pass

    def teardown(self) -> None:
        pass

    def get_gpu_memory_mb(self) -> int:
        return random.randint(4_000, 8_000)

    def get_kv_cache_hit_rate(self) -> float:
        return round(random.uniform(0.4, 0.95), 3)

    async def _send_request(self, prompt: str, max_tokens: int) -> RawRequestResult:
        await asyncio.sleep(0.05)  # simulate network + inference latency
        ttft_ms = round(random.uniform(80, 200), 2)
        itl_ms = round(random.uniform(10, 30), 2)
        output_tokens = max_tokens
        total_ms = ttft_ms + itl_ms * output_tokens
        return RawRequestResult(
            ttft_ms=ttft_ms,
            itl_ms=itl_ms,
            total_ms=total_ms,
            output_tokens=output_tokens,
        )
