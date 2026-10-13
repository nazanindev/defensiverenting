DELETE FROM advice_citations
WHERE advice_id = (SELECT id FROM advice WHERE slug = 'leave-owe-short')
  AND source_id = (SELECT id FROM sources WHERE url = 'https://michiganlegalhelp.org/resources/housing/tenant-rights-and-responsibilities');
