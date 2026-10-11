-- "What you may still owe" read as confusing (Nazanin 2026-10-11). The
-- statements under it are about leaving with no legal reason, so the
-- heading says that, and sets up "When you can leave early" as the other
-- case. A stage sits on the link row, outside the review hash, so no stamp
-- changes.
UPDATE topics
SET stages = array_replace(stages, 'What you may still owe', 'If you leave without a legal reason')
WHERE slug = 'breaking-lease';

UPDATE playbook_statements ps
SET stage = 'If you leave without a legal reason'
FROM playbooks pb JOIN topics t ON t.id = pb.topic_id
WHERE ps.playbook_id = pb.id AND t.slug = 'breaking-lease'
  AND ps.stage = 'What you may still owe';

-- Proposals still waiting carry the stage they will set.
UPDATE statement_proposals
SET proposed = replace(proposed::text, '"stage": "What you may still owe"', '"stage": "If you leave without a legal reason"')::jsonb
WHERE status IN ('pending', 'snoozed') AND proposed::text LIKE '%"stage": "What you may still owe"%';
