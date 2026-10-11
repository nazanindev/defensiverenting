DELETE FROM playbook_advice WHERE advice_id = (SELECT id FROM advice WHERE slug = 'money-example');
DELETE FROM advice WHERE slug = 'money-example';
