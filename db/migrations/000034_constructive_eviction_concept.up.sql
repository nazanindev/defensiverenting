-- Constructive eviction joins the reference layer (2026-08-31). The topic
-- (000032) gives it pages; this concept row is what puts the term on the
-- homepage's "Legal terms, explained" list and gives it a /c/ page assembling
-- each place's definition, once the tagged statements publish (ADR-012).
--
-- ON CONFLICT so the row can be applied to prod ahead of the deploy that runs
-- this migration there.
INSERT INTO concepts (slug, name, topic_id, definition) VALUES
    ('constructive-eviction', 'Constructive eviction',
     (SELECT id FROM topics WHERE slug = 'constructive-eviction'),
     'When bad conditions force you to move out, the law can treat it as if your landlord evicted you.')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, definition = EXCLUDED.definition;
