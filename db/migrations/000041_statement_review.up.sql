-- Review moves to the statement (ADR-018 D2). A statement carries who reviewed
-- it, when (last_reviewed_at, unused since migration 000001), and a hash of
-- what they reviewed: the body, its tags, and every citation's source, locator
-- and quote. The stamp counts only while the stored hash still matches the
-- content, so an edit to the words or the evidence returns the statement to
-- unreviewed with no code path that has to remember to clear anything.
ALTER TABLE statements
  ADD COLUMN reviewed_by   TEXT NOT NULL DEFAULT '',
  ADD COLUMN reviewed_hash TEXT NOT NULL DEFAULT '';

-- The one definition of "what was reviewed". Every writer of reviewed_hash
-- and every reader that judges a stamp valid goes through this view.
CREATE VIEW statement_review_hash AS
SELECT s.id AS statement_id,
       md5(s.body_md || E'\n' || COALESCE(co.slug, '') || E'\n' || COALESCE(tr.slug, '') || E'\n' ||
           COALESCE((SELECT string_agg(src.url || E'\t' || c.locator || E'\t' || c.quote, E'\n'
                                       ORDER BY src.url, c.locator, c.quote)
                     FROM citations c JOIN sources src ON src.id = c.source_id
                     WHERE c.statement_id = s.id), '')) AS hash
FROM statements s
LEFT JOIN concepts co ON co.id = s.concept_id
LEFT JOIN topics   tr ON tr.id = s.topic_ref;

-- Every statement on a page that is live today was reviewed as part of that
-- page when it was published (ADR-018 D3): the person who published is the
-- reviewer, and the publish stamp is the time. Without this, no live page
-- could be saved again until each of its statements was stamped by hand.
UPDATE statements s
   SET last_reviewed_at = COALESCE(pb.last_reviewed_at, pb.published_at, pb.updated_at),
       reviewed_by      = COALESCE(NULLIF(pb.updated_by, ''), 'page publish'),
       reviewed_hash    = h.hash
  FROM playbook_statements ps
  JOIN playbooks pb ON pb.id = ps.playbook_id,
       statement_review_hash h
 WHERE ps.statement_id = s.id AND h.statement_id = s.id AND pb.status = 'published';
