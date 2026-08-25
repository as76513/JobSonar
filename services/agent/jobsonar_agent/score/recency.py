"""Recency sub-score (Week 6). Linear decay to 0 over RECENCY_WINDOW_DAYS
-- linear, not exponential, so the score stays easy to explain ("half the
window old = half credit").
"""

from __future__ import annotations

from datetime import datetime, timezone

RECENCY_WINDOW_DAYS = 60


def fit(posted_at: datetime | None, first_seen_at: datetime | None, now: datetime | None = None) -> float:
    reference = posted_at or first_seen_at
    if reference is None:
        return 1.0  # no date at all -- don't penalise missing data

    now = now or datetime.now(timezone.utc)
    if reference.tzinfo is None:
        reference = reference.replace(tzinfo=timezone.utc)

    age_days = (now - reference).total_seconds() / 86400
    if age_days <= 0:
        return 1.0
    return max(0.0, 1.0 - age_days / RECENCY_WINDOW_DAYS)
