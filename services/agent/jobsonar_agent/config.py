import os


def env(key: str, default: str = "") -> str:
    v = os.environ.get(key)
    return v if v else default


POSTGRES_DSN = env(
    "POSTGRES_DSN",
    "postgres://jobsonar:jobsonar@localhost:5432/jobsonar?sslmode=disable",
)
OLLAMA_HOST = env("OLLAMA_HOST", "http://localhost:11434").rstrip("/")
EMBED_MODEL = env("EMBED_MODEL", "nomic-embed-text")
EMBED_DIM = int(env("EMBED_DIM", "768"))
EMBED_BATCH = int(env("EMBED_BATCH", "16"))
EMBED_BACKEND = env("EMBED_BACKEND", "ollama")  # ollama | fake
OTEL_CONSOLE = env("OTEL_CONSOLE", "") == "1"
