-- Backing quotes for the backfill page notes (ADR-016 A1/A9, 2026-10-12).
-- Not used: three withholding quotes that say less than the note (Minnesota
-- on protest strikes, Tennessee a news page, Florida a bar pamphlet) and two
-- ABA practitioner articles (lawyer articles are never sources).
-- Each quote was found by a research agent, copied by script from the
-- checker's own dump of the live source, and read against the note it backs.
-- A source is tagged with its place (a state, or united-states for a national
-- page) and backs that place's pages. A row is inserted only when its source
-- passes the kind and place rules, so an odd existing row is skipped rather
-- than failing the deploy.

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://lafla.org/wp-content/uploads/2024/01/How-to-Collect-a-Judgment_FINAL_English_2023.11.28.pdf', 'Legal Aid Foundation of Los Angeles', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'california'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'You will not automatically be paid the judgment amount. You must take steps to collect the money from the Judgment Debtor if they do not pay you voluntarily', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://lafla.org/wp-content/uploads/2024/01/How-to-Collect-a-Judgment_FINAL_English_2023.11.28.pdf' AND j.slug = 'california'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'You will not automatically be paid the judgment amount. You must take steps to collect the money from the Judgment Debtor if they do not pay you voluntarily');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://sanmateo.courts.ca.gov/self-help/small-claims-self-help/what-happens-after-trial', 'Superior Court of California, County of San Mateo', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'california'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'The court will not collect the money for you. But the court will issue the orders and other documents you may need to collect your judgment from the debtor', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://sanmateo.courts.ca.gov/self-help/small-claims-self-help/what-happens-after-trial' AND j.slug = 'california'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'The court will not collect the money for you. But the court will issue the orders and other documents you may need to collect your judgment from the debtor');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.peopleslawiowa.org/index.php/research-topics/consumer-law/small-claims/collecting-small-claims-judgment', 'Iowa People''s Law Library (State of Iowa)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'iowa'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Just because you win a judgment, does not mean the defendant will automatically pay you the money.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.peopleslawiowa.org/index.php/research-topics/consumer-law/small-claims/collecting-small-claims-judgment' AND j.slug = 'iowa'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Just because you win a judgment, does not mean the defendant will automatically pay you the money.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://alpenacounty.org/523/Civil-Small-Claims-Division-PDF', 'Alpena County Trial Court (Michigan)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'michigan'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'A judgment does not mean automatic payment.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://alpenacounty.org/523/Civil-Small-Claims-Division-PDF' AND j.slug = 'michigan'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'A judgment does not mean automatic payment.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.nycourts.gov/node/57021', 'New York State Unified Court System', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'new-york'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'The Court will not collect the money for you. You must collect the judgment yourself.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.nycourts.gov/node/57021' AND j.slug = 'new-york'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'The Court will not collect the money for you. You must collect the judgment yourself.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.clark.wa.gov/law-library/collecting-court-judgment', 'Clark County Law Library (Washington)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'washington'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'A money judgment in your favor does not necessarily mean that the money will be paid, but you can take steps to collect that money.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.clark.wa.gov/law-library/collecting-court-judgment' AND j.slug = 'washington'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'A money judgment in your favor does not necessarily mean that the money will be paid, but you can take steps to collect that money.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.lawhelpmn.org/sites/default/files/2022-01/H-13%20Condemnations.pdf', 'LawHelpMN (Minnesota legal aid)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'minnesota'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Unless the condemnation is stopped, you must move out by the date, or the police can remove you.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://www.lawhelpmn.org/sites/default/files/2022-01/H-13%20Condemnations.pdf' AND j.slug = 'minnesota'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Unless the condemnation is stopped, you must move out by the date, or the police can remove you.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.washingtonlawhelp.org/en/place-im-living-has-been-condemned', 'Washington Law Help (Northwest Justice Project)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'washington'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Sometimes the conditions are so serious that the government agency will condemn the rental unit or building, which means it’s illegal for your landlord to rent out any of the units or for anyone to live in the units.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://www.washingtonlawhelp.org/en/place-im-living-has-been-condemned' AND j.slug = 'washington'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Sometimes the conditions are so serious that the government agency will condemn the rental unit or building, which means it’s illegal for your landlord to rent out any of the units or for anyone to live in the units.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://coloradolegalservices.org/?p=20968', 'Colorado Legal Services', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'colorado'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Important: Do not stop paying rent while you are waiting for the landlord to make repairs. If you do, your landlord can evict you.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://coloradolegalservices.org/?p=20968' AND j.slug = 'colorado'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Important: Do not stop paying rent while you are waiting for the landlord to make repairs. If you do, your landlord can evict you.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://oag.maryland.gov/i-need-to/Pages/landlord-tenant-disputes.aspx', 'Maryland Office of the Attorney General', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'maryland'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you withhold rent without establishing escrow, you must still notify the landlord by certified mail, but they could try to evict you.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://oag.maryland.gov/i-need-to/Pages/landlord-tenant-disputes.aspx' AND j.slug = 'maryland'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'If you withhold rent without establishing escrow, you must still notify the landlord by certified mail, but they could try to evict you.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.mass.gov/doc/occupants-legal-rights-and-responsibilities-0/download', 'Massachusetts Department of Public Health', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'massachusetts'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Save the rent money you withhold. A judge may require you to pay all or some of it back. If they do and you don’t have this money, you may be evicted.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://www.mass.gov/doc/occupants-legal-rights-and-responsibilities-0/download' AND j.slug = 'massachusetts'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Save the rent money you withhold. A judge may require you to pay all or some of it back. If they do and you don’t have this money, you may be evicted.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.nj.gov/dca/codes/publications/pdf_lti/habitablity.pdf', 'New Jersey Department of Community Affairs', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'new-jersey'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Each case must be judged on it own facts. To avoid eviction, any rent withheld by the tenant should be saved and accessible in case the court requires the tenant to pay the outstanding rent.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://www.nj.gov/dca/codes/publications/pdf_lti/habitablity.pdf' AND j.slug = 'new-jersey'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Each case must be judged on it own facts. To avoid eviction, any rent withheld by the tenant should be saved and accessible in case the court requires the tenant to pay the outstanding rent.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.lawny.org/page/612/when-landlord-wont-make-repairs', 'LawNY (Legal Assistance of Western New York)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'new-york'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'When a rental unit has significant problems, tenants sometimes withhold rent or pay for the repairs and deduct the cost from their rent. This approach is very risky and often results in the landlord filing a nonpayment eviction against the tenant.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://www.lawny.org/page/612/when-landlord-wont-make-repairs' AND j.slug = 'new-york'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'When a rental unit has significant problems, tenants sometimes withhold rent or pay for the repairs and deduct the cost from their rent. This approach is very risky and often results in the landlord filing a nonpayment eviction against the tenant.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.pisgahlegal.org/wp-content/uploads/2026/02/Tenants-Right-to-a-Safe-and-Decent-Home.pdf', 'Pisgah Legal Services', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'north-carolina'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'However, if a tenant is behind on rent or is withholding rent from their landlord based on repair issues, the landlord may have grounds to evict a tenant for nonpayment of rent.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://www.pisgahlegal.org/wp-content/uploads/2026/02/Tenants-Right-to-a-Safe-and-Decent-Home.pdf' AND j.slug = 'north-carolina'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'However, if a tenant is behind on rent or is withholding rent from their landlord based on repair issues, the landlord may have grounds to evict a tenant for nonpayment of rent.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://zanesvillemunicipalcourt.org/renting.php', 'Zanesville Municipal Court', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'ohio'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'WARNING: If you do not follow proper procedure when withholding rent, you may be evicted.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://zanesvillemunicipalcourt.org/renting.php' AND j.slug = 'ohio'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'WARNING: If you do not follow proper procedure when withholding rent, you may be evicted.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://oregonlawhelp.org/media/174/download', 'Oregon Law Help', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'oregon'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you withhold rent without making a written request first, you could lose your housing.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://oregonlawhelp.org/media/174/download' AND j.slug = 'oregon'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'If you withhold rent without making a written request first, you could lose your housing.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://renters.equalhousing.org/repairs-security-deposit/repairs/', 'Housing Equality Center of Pennsylvania', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'pennsylvania'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If your landlord is not providing a habitable rental unit, he cannot evict you in retaliation for properly using one of these remedies. But he can still evict you for non-payment of rent if you withheld rent improperly or have violated the lease in other ways.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://renters.equalhousing.org/repairs-security-deposit/repairs/' AND j.slug = 'pennsylvania'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'If your landlord is not providing a habitable rental unit, he cannot evict you in retaliation for properly using one of these remedies. But he can still evict you for non-payment of rent if you withheld rent improperly or have violated the lease in other ways.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://texaslawhelp.org/form/repair-rights', 'TexasLawHelp (Austin Tenants Council repair rights guide)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'texas'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'A tenant who is behind in rent or withholds rent because repairs have not been made may be evicted and forfeits all rights to have repairs made until the rent is paid.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://texaslawhelp.org/form/repair-rights' AND j.slug = 'texas'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'A tenant who is behind in rent or withholds rent because repairs have not been made may be evicted and forfeits all rights to have repairs made until the rent is paid.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.justice4all.org/tenants-rights-information/', 'Legal Aid Justice Center', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'virginia'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'IMPORTANT: Under Virginia law, you cannot stop paying rent because of bad conditions, even if the problems are very serious. Withholding rent could lead to your eviction, and the poor conditions would not be a defense.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://www.justice4all.org/tenants-rights-information/' AND j.slug = 'virginia'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'IMPORTANT: Under Virginia law, you cannot stop paying rent because of bad conditions, even if the problems are very serious. Withholding rent could lead to your eviction, and the poor conditions would not be a defense.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.washingtonlawhelp.org/tenants-if-you-need-repairs', 'Washington LawHelp', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'washington'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'You should not withhold rent to try to make your landlord make repairs, or they may start the eviction process against you for not paying rent.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://www.washingtonlawhelp.org/tenants-if-you-need-repairs' AND j.slug = 'washington'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'You should not withhold rent to try to make your landlord make repairs, or they may start the eviction process against you for not paying rent.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.wislawhelp.org/page/120/i-withheld-or-want-withhold-rent-because-apartment-such-bad-condition', 'Wisconsin LawHelp', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'wisconsin'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Even if the court agrees that you could reduce your rent somewhat, if the official thinks you reduced the rent by too much, then you could be evicted.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'withhold-risk-evict' AND s.url = 'https://www.wislawhelp.org/page/120/i-withheld-or-want-withhold-rent-because-apartment-such-bad-condition' AND j.slug = 'wisconsin'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Even if the court agrees that you could reduce your rent somewhat, if the official thinks you reduced the rent by too much, then you could be evicted.');

