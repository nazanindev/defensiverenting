-- A statement can reference a whole topic instead of a concept. ADR-011 D7.
--
-- Two different relationships were hiding in "this statement generalizes
-- something". A concept marks a claim whose localized counterpart is another
-- STATEMENT ("landlords cannot retaliate" <-> Boston's Mass.-cited retaliation
-- statement). A topic reference marks a statement that is a one-paragraph
-- summary of a subject the site covers as entire PAGES ("your home must be
-- safe and fit to live in" is the repairs-and-habitability topic in
-- miniature). Forcing the second kind through concepts is what the tagging
-- pass tripped over: the fundamentals page's summary statements could carry
-- no concept without breaking topic ownership, because they are not claims,
-- they are tables of contents.
--
-- A statement carries at most one of the two. The CHECK enforces it: a
-- statement that is both a specific claim and a whole-subject summary is two
-- statements (ADR-003, atomicity).
ALTER TABLE statements ADD COLUMN topic_ref BIGINT REFERENCES topics(id);
ALTER TABLE statements ADD CONSTRAINT statements_one_tag_check
    CHECK (concept_id IS NULL OR topic_ref IS NULL);
