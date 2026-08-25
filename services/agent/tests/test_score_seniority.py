from jobsonar_agent.score.seniority import fit, infer_band


def test_infer_band_matches_common_titles():
    assert infer_band("Senior DevOps Engineer") == "senior"
    assert infer_band("Staff Platform Engineer") == "lead"
    assert infer_band("Junior Software Engineer") == "junior"
    assert infer_band("DevOps Engineer") == "mid"  # no level keyword -> default


def test_fit_no_preference_is_neutral():
    assert fit(None, "Junior Engineer") == 1.0
    assert fit("", "Junior Engineer") == 1.0


def test_fit_exact_match_is_one():
    assert fit("senior", "Senior DevOps Engineer") == 1.0


def test_fit_decays_with_band_distance():
    close = fit("senior", "Lead DevOps Engineer")  # 1 band away
    far = fit("senior", "Junior DevOps Engineer")  # 2 bands away
    assert 0 < far < close < 1.0


def test_fit_floors_at_zero_past_three_bands():
    assert fit("senior", "Intern DevOps Engineer") == 0.0  # 3 bands away


def test_fit_unrecognised_preference_is_neutral():
    assert fit("not-a-band", "Senior Engineer") == 1.0
