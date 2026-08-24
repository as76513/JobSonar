import math

from jobsonar_agent.resume.lexicon import SKILLS
from jobsonar_agent.resume.parse import normalize


class FakeEmbedder:
    """Lexicon-aligned vectors so overlap tracks skill match (no Ollama)."""

    def __init__(self, dim: int = 768):
        self.dim = dim

    def embed(self, texts: list[str]) -> list[list[float]]:
        out: list[list[float]] = []
        for text in texts:
            hay = " " + normalize(text) + " "
            vec = [0.0] * self.dim
            for i, skill in enumerate(SKILLS):
                needle = " " + normalize(skill) + " "
                if needle != "  " and needle in hay:
                    vec[i % self.dim] += 1.0
            if not any(vec):
                for word in hay.split():
                    vec[hash(word) % self.dim] += 1.0
            norm = math.sqrt(sum(x * x for x in vec)) or 1.0
            out.append([x / norm for x in vec])
        return out
