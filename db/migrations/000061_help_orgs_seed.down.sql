DELETE FROM help_orgs WHERE updated_by = 'audit 2026-10-04' AND host NOT IN (SELECT host FROM help_org_contacts);
