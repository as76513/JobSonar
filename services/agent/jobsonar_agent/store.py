from __future__ import annotations

import json
from collections.abc import Iterable

import psycopg

from jobsonar_agent import config


def _vec(values: Iterable[float]) -> str:
    return "[" + ",".join(f"{x:.8f}" for x in values) + "]"


class Store:
    def __init__(self, dsn: str | None = None):
        self.dsn = dsn or config.POSTGRES_DSN

    def connect(self):
        return psycopg.connect(self.dsn)

    def pending_resumes(self) -> list[dict]:
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                SELECT id::text, storage_uri
                FROM resumes
                WHERE status = 'pending'
                ORDER BY created_at
                """
            )
            return [{"id": r[0], "storage_uri": r[1]} for r in cur.fetchall()]

    def mark_resume(self, resume_id: str, status: str, parsed: dict | None, error: str = "") -> None:
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                UPDATE resumes
                SET status = %s, parsed = %s, error = %s
                WHERE id = %s::uuid
                """,
                (status, json.dumps(parsed) if parsed is not None else None, error, resume_id),
            )
            conn.commit()

    def upsert_skills(self, skills: list[str]) -> None:
        raw = json.dumps(skills)
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute("SELECT id FROM profiles ORDER BY updated_at DESC LIMIT 1")
            row = cur.fetchone()
            if row:
                cur.execute(
                    """
                    UPDATE profiles
                    SET skills = %s::jsonb, embedding = NULL, updated_at = now()
                    WHERE id = %s
                    """,
                    (raw, row[0]),
                )
            else:
                cur.execute("INSERT INTO profiles (skills) VALUES (%s::jsonb)", (raw,))
            conn.commit()

    def upsert_profile(self, skills: list[str], embedding: list[float]) -> None:
        raw = json.dumps(skills)
        vec = _vec(embedding)
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute("SELECT id FROM profiles ORDER BY updated_at DESC LIMIT 1")
            row = cur.fetchone()
            if row:
                cur.execute(
                    """
                    UPDATE profiles
                    SET skills = %s::jsonb, embedding = %s::vector, updated_at = now()
                    WHERE id = %s
                    """,
                    (raw, vec, row[0]),
                )
            else:
                cur.execute(
                    """
                    INSERT INTO profiles (skills, embedding)
                    VALUES (%s::jsonb, %s::vector)
                    """,
                    (raw, vec),
                )
            conn.commit()

    def profiles_missing_embedding(self) -> list[dict]:
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                SELECT id::text, skills
                FROM profiles
                WHERE embedding IS NULL
                """
            )
            out = []
            for rid, skills in cur.fetchall():
                if isinstance(skills, str):
                    skills = json.loads(skills)
                out.append({"id": rid, "skills": skills or []})
            return out

    def set_profile_embedding(self, profile_id: str, embedding: list[float]) -> None:
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                UPDATE profiles
                SET embedding = %s::vector, updated_at = now()
                WHERE id = %s::uuid
                """,
                (_vec(embedding), profile_id),
            )
            conn.commit()

    def jobs_missing_embeddings(self, limit: int) -> list[dict]:
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                SELECT j.id::text, j.title, j.description_md
                FROM jobs j
                LEFT JOIN job_embeddings e ON e.job_id = j.id
                WHERE e.job_id IS NULL
                ORDER BY j.last_seen_at DESC
                LIMIT %s
                """,
                (limit,),
            )
            return [
                {"id": r[0], "title": r[1] or "", "description_md": r[2] or ""}
                for r in cur.fetchall()
            ]

    def jobs_for_scoring(self, profile_id: str, limit: int) -> list[dict]:
        """Jobs missing a scores row, or re-scored since last_seen_at/
        profile.updated_at moved -- same "backfill what's stale" shape as
        jobs_missing_embeddings."""
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                SELECT j.id::text, j.title, j.description_md, j.location, j.remote_type,
                       j.posted_at, j.first_seen_at,
                       (1 - (e.embedding <=> p.embedding))::float8 AS semantic
                FROM jobs j
                LEFT JOIN job_embeddings e ON e.job_id = j.id
                JOIN profiles p ON p.id = %(profile_id)s::uuid
                LEFT JOIN scores s ON s.job_id = j.id AND s.profile_id = p.id
                WHERE s.job_id IS NULL
                   OR s.scored_at < j.last_seen_at
                   OR s.scored_at < p.updated_at
                ORDER BY j.last_seen_at DESC
                LIMIT %(limit)s
                """,
                {"profile_id": profile_id, "limit": limit},
            )
            cols = ("id", "title", "description_md", "location", "remote_type", "posted_at", "first_seen_at", "semantic")
            return [dict(zip(cols, row)) for row in cur.fetchall()]

    def upsert_score(
        self,
        job_id: str,
        profile_id: str,
        *,
        composite: float,
        skill_cov: float,
        semantic: float | None,
        seniority_fit: float,
        location_fit: float,
        recency: float,
        band: str,
        matched_skills: list[str],
        missing_skills: list[str],
    ) -> None:
        """Persists one job's sub-scores. The CASE below is the hard gate
        (CLAUDE.md golden rule 3 / FRD FR-12): must-have skills (jsonb
        containment), a clear seniority mismatch, or a clear location
        mismatch force band='excluded' regardless of the composite the
        agent computed -- decided by Postgres via SQL, not Python
        control flow, so a good composite/semantic score can never talk
        its way past a gate.
        """
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO scores (job_id, profile_id, composite, skill_cov, semantic,
                                     seniority_fit, location_fit, recency, band,
                                     matched_skills, missing_skills, scored_at)
                SELECT
                    %(job_id)s::uuid, %(profile_id)s::uuid, %(composite)s, %(skill_cov)s, %(semantic)s,
                    %(seniority_fit)s, %(location_fit)s, %(recency)s,
                    CASE
                        WHEN NOT (p.must_have_skills = '[]'::jsonb
                                  OR p.must_have_skills <@ COALESCE(j.skills_extracted, '[]'::jsonb))
                            THEN 'excluded'
                        WHEN COALESCE(p.seniority, '') <> '' AND %(seniority_fit)s < 0.3
                            THEN 'excluded'
                        WHEN (COALESCE(p.location, '') <> '' OR COALESCE(p.remote_pref, '') <> '')
                             AND %(location_fit)s = 0.0
                            THEN 'excluded'
                        ELSE %(band)s
                    END,
                    %(matched_skills)s::jsonb, %(missing_skills)s::jsonb, now()
                FROM jobs j, profiles p
                WHERE j.id = %(job_id)s::uuid AND p.id = %(profile_id)s::uuid
                ON CONFLICT (job_id, profile_id) DO UPDATE SET
                    composite = EXCLUDED.composite,
                    skill_cov = EXCLUDED.skill_cov,
                    semantic = EXCLUDED.semantic,
                    seniority_fit = EXCLUDED.seniority_fit,
                    location_fit = EXCLUDED.location_fit,
                    recency = EXCLUDED.recency,
                    band = EXCLUDED.band,
                    matched_skills = EXCLUDED.matched_skills,
                    missing_skills = EXCLUDED.missing_skills,
                    scored_at = EXCLUDED.scored_at
                """,
                {
                    "job_id": job_id,
                    "profile_id": profile_id,
                    "composite": composite,
                    "skill_cov": skill_cov,
                    "semantic": semantic,
                    "seniority_fit": seniority_fit,
                    "location_fit": location_fit,
                    "recency": recency,
                    "band": band,
                    "matched_skills": json.dumps(matched_skills),
                    "missing_skills": json.dumps(missing_skills),
                },
            )
            conn.commit()

    def set_job_skills_extracted(self, job_id: str, skills: list[str]) -> None:
        with self.connect() as conn, conn.cursor() as cur:
            cur.execute(
                "UPDATE jobs SET skills_extracted = %s::jsonb WHERE id = %s::uuid",
                (json.dumps(skills), job_id),
            )
            conn.commit()

    def upsert_job_embeddings(self, rows: list[tuple[str, list[float]]], model: str) -> None:
        if not rows:
            return
        with self.connect() as conn, conn.cursor() as cur:
            for job_id, embedding in rows:
                cur.execute(
                    """
                    INSERT INTO job_embeddings (job_id, embedding, model)
                    VALUES (%s::uuid, %s::vector, %s)
                    ON CONFLICT (job_id) DO UPDATE
                    SET embedding = EXCLUDED.embedding,
                        model = EXCLUDED.model,
                        updated_at = now()
                    """,
                    (job_id, _vec(embedding), model),
                )
            conn.commit()
