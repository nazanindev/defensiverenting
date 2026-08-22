-- Quiet enjoyment joins the concept registry. The claim was already stated on
-- the Boston, NYC, and national landlord-entry pages (each defining the term
-- inline), but it had no concept, so those statements sat untagged and the
-- coverage matrix could not see that two published Pennsylvania statements
-- make the same claim under the wrong name entirely ("warrant of
-- habitability" — that is the repairs doctrine, not the no-disturbance one).
-- A registry entry is what makes that kind of drift visible.
INSERT INTO concepts (slug, name, topic_id) VALUES
    ('quiet-enjoyment', 'Peaceful use of your home', (SELECT id FROM topics WHERE slug = 'landlord-entry'));

-- Name the term of art alongside the plain name, so an author or the drafting
-- agent searching for "warranty of habitability" finds the concept that
-- already models it instead of proposing a duplicate.
UPDATE concepts SET name = 'Right to a safe, livable home (warranty of habitability)'
    WHERE slug = 'habitability-standard';
