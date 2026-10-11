-- Arizona backs the short risk note (ADR-016 A1, option B). Found on a
-- second, deeper search; verified verbatim at its live source. Its second
-- quote says to speak with an attorney, not that a court may disagree, so
-- Arizona pages pair this note with the site-voice line.
INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://clsaz.org/wp-content/uploads/2026/07/Housing-Page_AZ-Landlord-Tenant-Booklet-English.pdf',
       'Community Legal Services (Arizona)', 'nonprofit', j.id
FROM jurisdictions j WHERE j.slug = 'arizona'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);

INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id,
       'If you move out before your rental agreement ends, your landlord may charge you for the remainder of the rental agreement until he re-rents the rental unit if allowed in your rental agreement.',
       10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND j.slug = 'arizona' AND s.kind IN ('gov_guidance', 'nonprofit')
  AND s.url = 'https://clsaz.org/wp-content/uploads/2026/07/Housing-Page_AZ-Landlord-Tenant-Booklet-English.pdf';
