from jobsonar_agent.score.location import fit


def test_no_preference_is_neutral():
    assert fit(None, None, "Pune, Maharashtra", "") == 1.0


def test_location_substring_match():
    assert fit("Pune", None, "Pune, Maharashtra", "") == 1.0


def test_location_mismatch():
    assert fit("Amsterdam", None, "Pune, Maharashtra", "") == 0.0


def test_remote_pref_match():
    assert fit(None, "remote", "Pune, Maharashtra", "Remote") == 1.0


def test_preference_set_but_job_has_no_data_is_neutral():
    assert fit("Pune", "remote", "", "") == 1.0


def test_mixed_signals_average():
    # location matches, remote pref does not -> 0.5
    assert fit("Pune", "remote", "Pune, Maharashtra", "onsite") == 0.5
