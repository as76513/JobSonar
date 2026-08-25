"""Seniority sub-score (Week 6). A job's band is inferred from its title
by keyword match -- same "deterministic, explainable" style as skill
extraction, not a model call. Unmatched titles default to "mid": most
postings that don't say otherwise are mid-level, and defaulting to the
middle band means an unspecified title is never a hard mismatch against
either end of a stated preference.
"""

from __future__ import annotations

BANDS = ("intern", "junior", "mid", "senior", "lead", "principal")

_BAND_KEYWORDS: tuple[tuple[str, tuple[str, ...]], ...] = (
    ("intern", ("intern", "internship")),
    ("junior", ("junior", "jr.", "jr ", "entry level", "entry-level", "associate")),
    ("principal", ("principal", "director", "vp ", "head of")),
    ("lead", ("lead", "staff", "architect")),
    ("senior", ("senior", "sr.", "sr ")),
)


def infer_band(title: str) -> str:
    lowered = f" {title.lower()} "
    for band, keywords in _BAND_KEYWORDS:
        for kw in keywords:
            if kw in lowered:
                return band
    return "mid"


def fit(profile_seniority: str | None, job_title: str) -> float:
    """1.0 if unset (no preference) or exact match; decays with band
    distance so a one-band gap (e.g. senior wanted, lead posted) still
    ranks well ahead of a three-band gap (e.g. senior wanted, intern
    posted)."""
    if not profile_seniority:
        return 1.0
    wanted = profile_seniority.strip().lower()
    if wanted not in BANDS:
        return 1.0  # unrecognised preference value -- don't penalise on it
    job_band = infer_band(job_title)
    distance = abs(BANDS.index(wanted) - BANDS.index(job_band))
    return max(0.0, 1.0 - distance * 0.35)
