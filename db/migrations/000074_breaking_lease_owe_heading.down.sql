UPDATE topics
SET stages = array_replace(stages, 'If you leave without a legal reason', 'What you may still owe')
WHERE slug = 'breaking-lease';

UPDATE playbook_statements ps
SET stage = 'What you may still owe'
FROM playbooks pb JOIN topics t ON t.id = pb.topic_id
WHERE ps.playbook_id = pb.id AND t.slug = 'breaking-lease'
  AND ps.stage = 'If you leave without a legal reason';

UPDATE statement_proposals
SET proposed = replace(proposed::text, '"stage": "If you leave without a legal reason"', '"stage": "What you may still owe"')::jsonb
WHERE status IN ('pending', 'snoozed') AND proposed::text LIKE '%"stage": "If you leave without a legal reason"%';
