-- Advice backing follows the page's place (ADR-016 A1, 2026-10-09): a page
-- uses its own state's source first, then a national one, never another
-- state's. The Pennsylvania Attorney General's guide is a Pennsylvania
-- source, so it backs advice on Pennsylvania pages only.
UPDATE sources s SET jurisdiction_id = j.id
FROM jurisdictions j
WHERE j.slug = 'pennsylvania'
  AND s.url = 'https://www.attorneygeneral.gov/wp-content/uploads/ConsumerTenant-Landlord-Guide.pdf';

-- A national backing for answering court papers, written for renters.
INSERT INTO sources (url, publisher, kind) VALUES
    ('https://nlihc.org/resource/evictions-101-eviction-process-how-it-works-and-what-know', 'National Low Income Housing Coalition', 'nonprofit')
ON CONFLICT (url) DO NOTHING;

INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id,
       'You MUST respond to the summons and/or show up to this hearing. If you do not show up, the judge will likely rule against you, even if you have a strong defense against eviction.',
       1
FROM advice a, sources s
WHERE a.slug = 'answer-court-papers'
  AND s.url = 'https://nlihc.org/resource/evictions-101-eviction-process-how-it-works-and-what-know';
