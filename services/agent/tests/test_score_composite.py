from jobsonar_agent.score.composite import band, composite


def test_composite_all_signals_present():
    c = composite(skill_cov=1.0, semantic=1.0, seniority_fit=1.0, location_fit=1.0, recency=1.0)
    assert c == 1.0


def test_composite_missing_semantic_renormalises_not_zeros():
    with_semantic = composite(1.0, 1.0, 1.0, 1.0, 1.0)
    without_semantic = composite(1.0, None, 1.0, 1.0, 1.0)
    assert with_semantic == without_semantic == 1.0  # all other signals perfect either way


def test_composite_weights_skill_cov_highest():
    # Same total "distance from perfect," but concentrated on skill_cov vs.
    # spread elsewhere -- skill_cov should hurt the composite more.
    hurt_skill_cov = composite(0.0, 1.0, 1.0, 1.0, 1.0)
    hurt_recency = composite(1.0, 1.0, 1.0, 1.0, 0.0)
    assert hurt_skill_cov < hurt_recency


def test_bands_match_frd_thresholds():
    assert band(0.9) == "strong"
    assert band(0.70) == "strong"
    assert band(0.5) == "possible"
    assert band(0.1) == "stretch"
