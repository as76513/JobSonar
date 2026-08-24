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
