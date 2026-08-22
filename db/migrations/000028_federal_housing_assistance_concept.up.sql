-- Federal housing assistance: the extra rules that apply in public housing,
-- with a voucher, or in other HUD assisted homes. The national drafts state
-- it under four topics already (entry rules, the CARES Act eviction notice,
-- HUD housing standards, the federally backed mortgage carve-outs), which
-- makes it a concept, and a cross-cutting one.
INSERT INTO concepts (slug, name, topic_id, definition) VALUES
    ('federal-housing-assistance', 'Federal housing assistance',
     (SELECT id FROM topics WHERE slug = 'renting-fundamentals'),
     'Extra federal rules that protect you in public housing, with a voucher, or in other HUD assisted homes.');
