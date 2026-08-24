from __future__ import annotations

import argparse
import logging
import time

from jobsonar_agent import config
from jobsonar_agent.embed.fake import FakeEmbedder
from jobsonar_agent.embed.ollama import OllamaEmbedder
from jobsonar_agent.llm import Embedder
from jobsonar_agent.otel import tracer
from jobsonar_agent.resume.parse import parse_resume
from jobsonar_agent.store import Store

log = logging.getLogger("jobsonar.agent")


def profile_text(skills: list[str]) -> str:
    return " ".join(skills)


def job_text(title: str, description: str) -> str:
    return f"{title}\n{description}".strip() or title or "job"


def parse_pending(store: Store, embedder: Embedder) -> int:
    n = 0
    for row in store.pending_resumes():
        rid = row["id"]
        with tracer().start_as_current_span("resume.parse") as span:
            span.set_attribute("resume.id", rid)
            try:
                parsed = parse_resume(row["storage_uri"])
                skills = parsed.get("skills") or []
                if not skills:
                    store.mark_resume(rid, "error", parsed, "no skills matched lexicon")
                    log.info("resume %s: no skills matched", rid)
                    continue
                store.mark_resume(rid, "done", parsed, "")
                span.set_attribute("resume.skill_count", len(skills))
                try:
                    vecs = embedder.embed([profile_text(skills)])
                    store.upsert_profile(skills, vecs[0])
                except Exception as exc:
                    store.upsert_skills(skills)
                    log.warning("resume %s: parsed, embed failed (%s)", rid, type(exc).__name__)
                log.info("resume %s: parsed %d skills", rid, len(skills))
                n += 1
            except Exception as exc:
                store.mark_resume(rid, "error", None, type(exc).__name__)
                log.warning("resume %s: %s", rid, type(exc).__name__)
    return n


def embed_profiles(store: Store, embedder: Embedder) -> int:
    rows = [p for p in store.profiles_missing_embedding() if p["skills"]]
    if not rows:
        return 0
    vecs = embedder.embed([profile_text(p["skills"]) for p in rows])
    for p, vec in zip(rows, vecs):
        store.set_profile_embedding(p["id"], vec)
    log.info("embedded %d profiles", len(rows))
    return len(rows)


def embed_jobs(store: Store, embedder: Embedder) -> int:
    total = 0
    while True:
        batch = store.jobs_missing_embeddings(config.EMBED_BATCH)
        if not batch:
            break
        texts = [job_text(j["title"], j["description_md"]) for j in batch]
        vecs = embedder.embed(texts)
        store.upsert_job_embeddings(
            [(j["id"], v) for j, v in zip(batch, vecs)],
            config.EMBED_MODEL,
        )
        total += len(batch)
        log.info("embedded %d jobs (batch)", len(batch))
    return total


def once(store: Store | None = None, embedder: Embedder | None = None) -> dict[str, int]:
    store = store or Store()
    if embedder is None:
        embedder = FakeEmbedder() if config.EMBED_BACKEND == "fake" else OllamaEmbedder()
    with tracer().start_as_current_span("agent.pass"):
        parsed = parse_pending(store, embedder)
        profiles = embed_profiles(store, embedder)
        jobs = embed_jobs(store, embedder)
    return {"resumes": parsed, "profiles": profiles, "jobs": jobs}


def main(argv: list[str] | None = None) -> int:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    parser = argparse.ArgumentParser(description="JobSonar local embed/parse worker")
    parser.add_argument("--once", action="store_true", help="one drain-and-exit pass")
    args = parser.parse_args(argv)
    if args.once:
        counts = once()
        log.info("done resumes=%s profiles=%s jobs=%s", counts["resumes"], counts["profiles"], counts["jobs"])
        return 0
    while True:
        try:
            once()
        except Exception as exc:
            log.warning("agent pass failed: %s", type(exc).__name__)
        time.sleep(1)


if __name__ == "__main__":
    raise SystemExit(main())
