"""Composite score + confidence band (Week 6).

Weights live here, named, in one place -- not scattered across the
pipeline -- because Week 6's P1 is making them tunable without editing
code. skill_cov is weighted highest: it's the primary "why this job"
signal surfaced in the UI today (semantic is explicitly a secondary
tiebreak per web/src/pages/Jobs.jsx's own copy).

Bands are named per FRD.md FR-14 ("strong / possible / stretch"), not
invented here.
"""

from __future__ import annotations

WEIGHTS = {
    "skill_cov": 0.40,
    "semantic": 0.20,
    "seniority_fit": 0.15,
    "location_fit": 0.15,
    "recency": 0.10,
}

BAND_THRESHOLDS = (
    ("strong", 0.70),
    ("possible", 0.45),
    # anything below the lowest threshold is "stretch"
)


def composite(skill_cov: float, semantic: float | None, seniority_fit: float, location_fit: float, recency: float) -> float:
    """Weighted average, renormalised over whichever sub-scores are
    actually available -- semantic is None until a job has an embedding
    (Week 5), and a missing sub-score should not silently count as a 0."""
    values = {
        "skill_cov": skill_cov,
        "semantic": semantic,
        "seniority_fit": seniority_fit,
        "location_fit": location_fit,
        "recency": recency,
    }
    present = {k: v for k, v in values.items() if v is not None}
    total_weight = sum(WEIGHTS[k] for k in present)
    if total_weight == 0:
        return 0.0
    return sum(WEIGHTS[k] * v for k, v in present.items()) / total_weight


def band(composite_score: float) -> str:
    for name, threshold in BAND_THRESHOLDS:
        if composite_score >= threshold:
            return name
    return "stretch"
