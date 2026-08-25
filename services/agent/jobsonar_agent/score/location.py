"""Location/remote sub-score (Week 6). Deliberately simple substring
matching, not geocoding -- explainable over precise, matching the rest of
this scoring layer. Averages whichever preference axes (remote type,
location text) the profile actually set and the job actually has data
for; an axis with nothing to compare against is dropped rather than
counted as a mismatch, since most connectors don't populate remote_type
today.
"""

from __future__ import annotations

import re

_norm_re = re.compile(r"[^a-z0-9]+")


def _norm(s: str) -> str:
    return _norm_re.sub(" ", s.lower()).strip()


def fit(profile_location: str | None, profile_remote_pref: str | None, job_location: str, job_remote_type: str) -> float:
    if not profile_location and not profile_remote_pref:
        return 1.0  # no preference set

    signals: list[float] = []

    if profile_remote_pref and job_remote_type:
        want, have = _norm(profile_remote_pref), _norm(job_remote_type)
        signals.append(1.0 if want in have or have in want else 0.0)

    if profile_location and job_location:
        want, have = _norm(profile_location), _norm(job_location)
        signals.append(1.0 if want in have else 0.0)

    if not signals:
        return 1.0  # preference set, but nothing on the job side to compare -- neutral, not a penalty

    return sum(signals) / len(signals)
