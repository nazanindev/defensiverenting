-- The 2026-09-11 back-fill filed reviewer notes through a path that
-- superseded any pending proposal on the same key, so the 21 statements with
-- two or more notes kept only their last one. Notes no longer supersede each
-- other (ADR-018 D1). Put the lost ones back; nothing was decided on them.
BEGIN;
UPDATE statement_proposals
   SET status = 'pending'
 WHERE reason = 'agent-pass:flag' AND status = 'superseded' AND decided_at IS NULL;
SELECT status, count(*) FROM statement_proposals WHERE reason = 'agent-pass:flag' GROUP BY status;
COMMIT;
