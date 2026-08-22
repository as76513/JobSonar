-- Target boards that actually post DevOps-family roles in Pune.
-- Greenhouse is per-company (board_token), not a keyword search —
-- Stripe's public board has Amsterdam/Remote infra, not Pune DevOps.
-- Applied by `make seed`. Safe to re-run.
INSERT INTO companies (name, ats, board_token)
VALUES
    ('Energy Exemplar', 'greenhouse', 'energyexemplarllc'),
    ('Tech Holding', 'greenhouse', 'techholding')
ON CONFLICT (ats, board_token) DO NOTHING;

-- Drop the Week 3 Stripe demo token if it is still around; it does not
-- match a Pune DevOps search.
DELETE FROM companies
WHERE ats = 'greenhouse' AND board_token = 'stripe';
