-- Utility shutoff protection (2026-08-31). The claim recurs wherever a heat
-- page meets unpaid utility bills: the nationwide cold-weather and hot-weather
-- shutoff rules, Pennsylvania's winter termination, medical certificate and
-- pay-and-keep-service rules, and the city heat pages. It was living untagged
-- on the pages.
--
-- ON CONFLICT so the row can be applied to prod ahead of the deploy that runs
-- this migration there.
INSERT INTO concepts (slug, name, topic_id, definition) VALUES
    ('utility-shutoff-protection', 'Utility shutoff protection',
     (SELECT id FROM topics WHERE slug = 'heat-not-working'),
     'Rules that limit when a utility company can shut off your water, electric, or gas, including cold and hot weather protections, and help paying the bill.')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, definition = EXCLUDED.definition;
