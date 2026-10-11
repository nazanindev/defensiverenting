DELETE FROM playbook_advice a USING playbook_advice b
WHERE a.playbook_id = b.playbook_id AND a.advice_id = b.advice_id AND a.ctid > b.ctid;
DROP INDEX IF EXISTS playbook_advice_ref;
ALTER TABLE playbook_advice ADD PRIMARY KEY (playbook_id, advice_id);
