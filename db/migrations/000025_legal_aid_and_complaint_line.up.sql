-- "Legal aid" is what renters, courts, and the organisations themselves call
-- it; "Free legal help" was a description of the same concept wearing a
-- different label, which is exactly the drift the term-of-art renames
-- (migration 000022) exist to prevent. Rename rather than duplicate; the
-- slug, being a public anchor, stays free-legal-help.
UPDATE concepts SET name = 'Legal aid' WHERE slug = 'free-legal-help';

-- Complaint line: the claim "here is the government line where you report
-- your landlord" recurs on nearly every page under a different agency name
-- (a fair housing commission, a housing department, an attorney general's
-- hotline, a state human relations agency). Cross-cutting, so any topic's
-- page can carry it. Distinct from hotline-211 (a referral service, not an
-- enforcement agency) and code-inspection (requesting an inspection, not
-- filing a complaint about the landlord's conduct).
INSERT INTO concepts (slug, name, topic_id, definition) VALUES
    ('complaint-line', 'Complaint line',
     (SELECT id FROM topics WHERE slug = 'renting-fundamentals'),
     'A government office that takes complaints about your landlord, like a housing department or a fair housing agency.');
