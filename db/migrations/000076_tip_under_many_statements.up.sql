-- A tip can sit under more than one statement on a page: a page with three
-- money examples shows the money-example tip under each (Nazanin
-- 2026-10-11). A page note is still once per page.
ALTER TABLE playbook_advice DROP CONSTRAINT playbook_advice_pkey;
CREATE UNIQUE INDEX playbook_advice_ref ON playbook_advice
    (playbook_id, advice_id, COALESCE(statement_key, '00000000-0000-0000-0000-000000000000'::uuid));
