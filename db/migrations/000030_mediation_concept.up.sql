-- Mediation: settling with the landlord through a neutral go-between instead
-- of, or before, court. The corpus states it as Just Mediation Pittsburgh,
-- Chicago's Early Resolution Program and judge-referred mediation, Seattle's
-- dispute resolution centers, and Philadelphia's eviction diversion program —
-- one claim, many local doors. Cross-cutting: it appears under cant-pay,
-- eviction, and the resource directories alike.
INSERT INTO concepts (slug, name, topic_id, definition) VALUES
    ('mediation', 'Mediation',
     (SELECT id FROM topics WHERE slug = 'renting-fundamentals'),
     'Working out a deal with your landlord through a neutral go-between, instead of or before court.');
