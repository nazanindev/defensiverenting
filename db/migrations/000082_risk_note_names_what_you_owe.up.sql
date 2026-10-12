-- The risk note says what a renter can owe, not just "it" (Nazanin
-- 2026-10-12): a landlord cannot charge for anything they like. Each state's
-- quote already says rent for the rest of the lease, or more; Virginia and
-- West Virginia get a second quote that says so, verified verbatim at the
-- live source.
UPDATE advice
SET body_md = 'Even with a good reason to leave, your landlord can sue you for rent. If a court disagrees with you, you can still owe rent after you move out, up to the end of your lease.'
WHERE slug = 'leave-risk-owe';

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://legalaidwv.org/legal-information/renting-a-home-or-apartment-understanding-your-lease/',
       'Legal Aid of West Virginia', 'nonprofit', j.id
FROM jurisdictions j WHERE j.slug = 'west-virginia'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);

INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id,
       'Even if you follow the process correctly, your landlord can still take you to court and try to hold you responsible for rent for the rest of the lease period.',
       12
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND j.slug = 'virginia' AND s.kind IN ('gov_guidance', 'nonprofit')
  AND s.url = 'https://www.virginialawhelp.org/en/can-i-end-my-lease-early-because-bad-conditions';

INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id,
       'Breaking a lease early may mean you are responsible for paying fees or a specific amount of rent until the unit is rented unless you are leaving for a good reason like the landlord won’t repair things that make the apartment unsafe to live in.',
       12
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND j.slug = 'west-virginia' AND s.kind IN ('gov_guidance', 'nonprofit')
  AND s.url = 'https://legalaidwv.org/legal-information/renting-a-home-or-apartment-understanding-your-lease/';
