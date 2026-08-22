-- Two cross-cutting concepts the corpus already states everywhere without a
-- tag. Small claims court is the enforcement path nearly every topic ends at,
-- and it is sharply place-variable: Texas calls it justice court, Pennsylvania
-- a magisterial district judge, Washington the district court small claims
-- docket. Records/evidence is the keep-proof claim (photos, letters, logs)
-- that every topic repeats and courts weigh differently. Both owned by
-- renting-fundamentals so any topic's page can carry them. Distinct from
-- eviction-record, which is about sealing the case file, not building one.
INSERT INTO concepts (slug, name, topic_id) VALUES
    ('small-claims-court',   'Small claims court',       (SELECT id FROM topics WHERE slug = 'renting-fundamentals')),
    ('records-and-evidence', 'Court records / evidence', (SELECT id FROM topics WHERE slug = 'renting-fundamentals'));
