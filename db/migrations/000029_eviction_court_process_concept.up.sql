-- Eviction court process: how the case itself runs, from the landlord filing
-- to the officer who carries out a removal, including the post-judgment
-- clock and the appeal. Distinct from answer-the-case (the renter's
-- response) and court-eviction-only (that a case is required at all). The
-- corpus walks this arc on nearly every eviction page in local terms:
-- justice court and 5-day appeals in Texas, magisterial district judges and
-- supersedeas deposits in Pennsylvania, writs of possession in Philadelphia,
-- up to a year to move in New York.
INSERT INTO concepts (slug, name, topic_id, definition) VALUES
    ('eviction-court-process', 'Eviction court process',
     (SELECT id FROM topics WHERE slug = 'eviction-defense'),
     'How an eviction case runs in court, from filing to hearing to the officer who carries out a removal.');
