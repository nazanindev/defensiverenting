DELETE FROM playbook_advice WHERE advice_id = (SELECT id FROM advice WHERE slug = 'keep-a-record');
DELETE FROM advice WHERE slug = 'keep-a-record';
