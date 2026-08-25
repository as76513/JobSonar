from datetime import datetime, timedelta, timezone

from jobsonar_agent.score.recency import RECENCY_WINDOW_DAYS, fit


def test_no_date_is_neutral():
    assert fit(None, None) == 1.0


def test_posted_today_is_full_score():
    now = datetime(2026, 1, 1, tzinfo=timezone.utc)
    assert fit(now, None, now=now) == 1.0


def test_decays_linearly_within_window():
    now = datetime(2026, 1, 1, tzinfo=timezone.utc)
    half_window_ago = now - timedelta(days=RECENCY_WINDOW_DAYS / 2)
    score = fit(half_window_ago, None, now=now)
    assert 0.49 <= score <= 0.51


def test_zero_past_the_window():
    now = datetime(2026, 1, 1, tzinfo=timezone.utc)
    long_ago = now - timedelta(days=RECENCY_WINDOW_DAYS * 3)
    assert fit(long_ago, None, now=now) == 0.0


def test_falls_back_to_first_seen_at():
    now = datetime(2026, 1, 1, tzinfo=timezone.utc)
    assert fit(None, now, now=now) == 1.0
