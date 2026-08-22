-- Late fees: when a landlord can charge one and how big it can be. Already
-- stated on the New York, Boston, and Austin cant-pay-rent pages with three
-- different answers, which is the definition of a concept. Owned by
-- cant-pay-rent, where the claim lives.
INSERT INTO concepts (slug, name, topic_id, definition) VALUES
    ('late-fees', 'Late fees',
     (SELECT id FROM topics WHERE slug = 'cant-pay-rent'),
     'What a landlord can charge when rent is late, and when the fee can start.');
