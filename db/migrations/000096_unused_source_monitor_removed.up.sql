-- The unused source monitor is removed (2026-11-13). The source check no
-- longer files a "delete this source" item for a source no page cites: such
-- a source reaches no reader and fails no check, and the items sat undecided
-- in the queue for months. The ones still waiting are closed as kept. No
-- source row is deleted.
UPDATE source_proposals
   SET status        = 'rejected',
       decided_by    = 'review agent',
       decided_at    = NOW(),
       decision_note = 'Unused source monitor removed 2026-11-13; the source is kept.',
       snoozed_until = NULL
 WHERE status IN ('pending', 'snoozed');
