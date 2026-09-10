from .base import BaseEngine, EngineStartError, EngineNotReadyError
from .vllm_engine import VLLMEngine
from .sglang_engine import SGLangEngine
from .llamacpp_engine import LlamaCppEngine
from .ollama_engine import OllamaEngine
from .mock_engine import MockEngine

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
