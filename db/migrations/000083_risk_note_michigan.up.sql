-- Michigan backs the short risk note (ADR-016 A1, option B). Its page was
-- held when the other states were researched, because Michigan's
-- legislature site could not be read. Verified verbatim at the live source
-- 2026-10-12. The quote says the renter can owe rent, not that a court may
-- disagree, so Michigan pages pair this note with the site-voice line.
INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://michiganlegalhelp.org/resources/housing/tenant-rights-and-responsibilities',
       'Michigan Legal Help', 'nonprofit', j.id
FROM jurisdictions j WHERE j.slug = 'michigan'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);

INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id,
       'If you need to move out before the end of your lease, you are responsible for paying rent for the rest of the term of the lease unless you moved out after the landlord did not meet a significant legal responsibility.',
       10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND j.slug = 'michigan' AND s.kind IN ('gov_guidance', 'nonprofit')
  AND s.url = 'https://michiganlegalhelp.org/resources/housing/tenant-rights-and-responsibilities';
