-- Week 4 default skill list so the keyword score can rank before resume upload (Week 5).
-- Safe to re-run: keeps a single profile.
INSERT INTO profiles (skills)
SELECT '["kubernetes","terraform","aws","azure","docker","devops","ci/cd","linux","python","go"]'::jsonb
WHERE NOT EXISTS (SELECT 1 FROM profiles);
