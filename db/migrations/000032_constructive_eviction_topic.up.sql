-- Constructive eviction gets its own page (2026-08-31). The concept was
-- explained inside repairs/habitability statements; it is the term a renter
-- hears from legal aid and then searches for, and the claim carries enough
-- risk (move out first, prove it in court later) to deserve the full
-- treatment rather than a paragraph on another page.
--
-- Same shape as 000014: this migration is the authority on the name.
INSERT INTO topics (slug, name, is_core) VALUES
    ('constructive-eviction', 'Constructive Eviction', false)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, is_core = EXCLUDED.is_core;
