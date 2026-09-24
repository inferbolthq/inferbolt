from .base import BaseEngine, EngineNotReadyError, EngineStartError
from .llamacpp_engine import LlamaCppEngine
from .mock_engine import MockEngine
from .ollama_engine import OllamaEngine
from .sglang_engine import SGLangEngine
from .vllm_engine import VLLMEngine

# Re-exported so callers import the engine contract and its errors from one place.
__all__ = [
    "ENGINES",
    "BaseEngine",
    "EngineNotReadyError",
    "EngineStartError",
    "LlamaCppEngine",
    "MockEngine",
    "OllamaEngine",
    "SGLangEngine",
    "VLLMEngine",
    "get_engine",
]

ENGINES: dict[str, type[BaseEngine]] = {
    "vllm": VLLMEngine,
    "sglang": SGLangEngine,
    "llamacpp": LlamaCppEngine,
    "ollama": OllamaEngine,
    "mock": MockEngine,
}


def get_engine(name: str) -> BaseEngine:
    if name not in ENGINES:
        raise ValueError(f"Unknown engine: {name}. Available: {list(ENGINES.keys())}")
    return ENGINES[name]()
