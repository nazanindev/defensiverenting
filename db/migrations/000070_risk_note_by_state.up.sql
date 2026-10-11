-- ADR-016 A1, risk note option B (Nazanin, 2026-10-11).
-- A state whose source says both that a renter who leaves can owe or be sued
-- for rent and that a reason to leave can fail in court backs the full note,
-- leave-risk-owe. A state whose source says only that the renter can still owe
-- backs the short note, leave-owe-short, which pages pair with the site-voice
-- line leave-court-may-disagree. Each quote was found by a research agent,
-- verified verbatim at its live source, and read against the note it backs.
-- A state with neither keeps its warnings on its statements.

INSERT INTO advice (slug, kind, warns, site_voice, body_md) VALUES
    ('leave-owe-short', 'page_note', 'owe', false, 'If you leave early, you can still owe rent after you move out, up to the end of your lease.'),
    ('leave-court-may-disagree', 'page_note', NULL, true, 'Even with a good reason to leave, a court might disagree.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.alabamalegalhelp.org/resource/ending-your-lease-and-moving-out', 'Legal Services Alabama (AlabamaLegalHelp.org)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'alabama'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Sometimes, you have to move before your lease has ended. When you do, you risk having your landlord sue you for rent for the whole lease term.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.alabamalegalhelp.org/resource/ending-your-lease-and-moving-out' AND j.slug = 'alabama' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.alabamalegalhelp.org/resource/ending-your-lease-and-moving-out', 'Legal Services Alabama (AlabamaLegalHelp.org)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'alabama'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'You can show that your landlord broke the lease and gave you good reason to move. The law is not always clear on when this will work, so don''t break your lease unless things are so bad that you have no other choice.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.alabamalegalhelp.org/resource/ending-your-lease-and-moving-out' AND j.slug = 'alabama' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.dre.ca.gov/publications/ResourceGuidebook/gb09_dealingwith.html', 'California Department of Real Estate', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'california'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'The defects may not affect the tenant’s health and safety seriously enough to justify using the remedy. The landlord may sue the tenant to collect additional rent or damages.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.dre.ca.gov/publications/ResourceGuidebook/gb09_dealingwith.html' AND j.slug = 'california' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.dre.ca.gov/publications/ResourceGuidebook/gb09_dealingwith.html', 'California Department of Real Estate', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'california'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Before the tenant abandons the rental property, it is a good idea to check with a legal aid organization, lawyer, housing clinic, or tenant program to help determine if rent withholding is the appropriate remedy.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.dre.ca.gov/publications/ResourceGuidebook/gb09_dealingwith.html' AND j.slug = 'california' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://attorneygeneral.delaware.gov/wp-content/uploads/sites/50/2019/06/NEW-Revised-LL-T-summary-for-tenants-4-1-19.pdf', 'Delaware Department of Justice (Landlord-Tenant Code summary)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'delaware'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'if a tenant terminates wrongfully, the tenant shall remain obligated under the lease', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://attorneygeneral.delaware.gov/wp-content/uploads/sites/50/2019/06/NEW-Revised-LL-T-summary-for-tenants-4-1-19.pdf' AND j.slug = 'delaware' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://attorneygeneral.delaware.gov/wp-content/uploads/sites/50/2019/06/NEW-Revised-LL-T-summary-for-tenants-4-1-19.pdf', 'Delaware Department of Justice (Landlord-Tenant Code summary)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'delaware'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If the tenant withholds rent or deducts from the rent and the Court finds that the withholding or deduction was wrongful, then the Court can grant the landlord either possession of the rented unit or a judgment for the amount withheld', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://attorneygeneral.delaware.gov/wp-content/uploads/sites/50/2019/06/NEW-Revised-LL-T-summary-for-tenants-4-1-19.pdf' AND j.slug = 'delaware' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://dca.georgia.gov/document/manuals/georgia-landlord-tenant-handbook-english/download', 'Georgia Department of Community Affairs', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'georgia'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'CAUTION! If you abandon the unit by moving out and you do not have a defense of constructive eviction, the landlord may: 1. End the lease; 2. Find another tenant while still holding you responsible for any money owed on your remaining lease term; or 3. Leave the unit empty and continue to collect rent from you according to the lease.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://dca.georgia.gov/document/manuals/georgia-landlord-tenant-handbook-english/download' AND j.slug = 'georgia' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.lawhelpmn.org/self-help-library/fact-sheet/notices-vacate-and-ending-lease', 'LawHelp Minnesota (Education for Justice)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'minnesota'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Warning! If you break your lease without the written agreement of the landlord, you may have to pay rent for the whole period of the lease. The landlord has to try to re-rent but you may have to pay until they find someone else.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.lawhelpmn.org/self-help-library/fact-sheet/notices-vacate-and-ending-lease' AND j.slug = 'minnesota' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.lawhelpmn.org/self-help-library/fact-sheet/getting-landlord-make-repairs', 'LawHelp Minnesota (Education for Justice)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'minnesota'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Repair problems are usually not a good reason to end a lease. If you leave your property and stop paying rent your landlord could take you to court. But very serious repair problems might give you the legal right to break your lease. It’s best to have a judge rule that the lease can be broken.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.lawhelpmn.org/self-help-library/fact-sheet/getting-landlord-make-repairs' AND j.slug = 'minnesota' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.mslegalservices.org/resource/landlord-tenant-law-in-mississippi', 'Mississippi Center for Legal Services and North Mississippi Rural Legal Services (MS Legal Services)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'mississippi'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If the landlord has not agreed to release the tenant from his rental obligation and has not breached the lease, the tenant is liable for the rent for the remaining period of the lease.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.mslegalservices.org/resource/landlord-tenant-law-in-mississippi' AND j.slug = 'mississippi' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.mslegalservices.org/resource/landlord-tenant-law-in-mississippi', 'Mississippi Center for Legal Services and North Mississippi Rural Legal Services (MS Legal Services)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'mississippi'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Warning: If the problem was caused by a tenant or his guest, or if notice was not properly given, a tenant may be in breach of the lease by canceling or using repair and deduct.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.mslegalservices.org/resource/landlord-tenant-law-in-mississippi' AND j.slug = 'mississippi' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.603legalaid.org/self-help-articles/housing/tenants/leases-rent-security-deposits/braking-a-lease/', '603 Legal Aid', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'new-hampshire'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you leave before you have paid all the rent for the year, and it is not because of anything the landlord did or did not do, you have breached the lease. The landlord can then try to get you to pay all the rent for the rest of the year, even though you are no longer living there.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.603legalaid.org/self-help-articles/housing/tenants/leases-rent-security-deposits/braking-a-lease/' AND j.slug = 'new-hampshire' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.603legalaid.org/self-help-articles/housing/tenants/leases-rent-security-deposits/braking-a-lease/', '603 Legal Aid', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'new-hampshire'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Even if you feel completely justified, you will still face the risk of owing money and possibly a court judgment against you.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.603legalaid.org/self-help-articles/housing/tenants/leases-rent-security-deposits/braking-a-lease/' AND j.slug = 'new-hampshire' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://texaslawhelp.org/article/lease-termination-ending-your-lease', 'Texas Law Help (Texas RioGrande Legal Aid and Texas Legal Services Center)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'texas'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If a tenant does not have a legal excuse to end the lease early, they may be responsible for rent until the lease expires or until the landlord finds a new tenant. The tenant may also owe any fees the lease mentions. Failure to pay can hurt the tenant’s credit, result in lawsuits, and may also show up on the tenant’s rental history.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://texaslawhelp.org/article/lease-termination-ending-your-lease' AND j.slug = 'texas' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://texaslawhelp.org/article/lease-termination-ending-your-lease', 'Texas Law Help (Texas RioGrande Legal Aid and Texas Legal Services Center)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'texas'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Likewise, a tenant may be able to break the lease if the landlord fails to keep the home repaired and habitable or harasses the tenant. The tenant will likely need a court order to break the lease, even if the landlord is at fault.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://texaslawhelp.org/article/lease-termination-ending-your-lease' AND j.slug = 'texas' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.utahlegalservices.org/node/7/bad-housing', 'Utah Legal Services', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'utah'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If your landlord refuses to fix major problems in your housing, you can break your lease and move. If you do this, the landlord may take you to court for rent they believe you still owe. But, if the conditions were so bad you could not live there, a judge may find that you don’t owe anything.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.utahlegalservices.org/node/7/bad-housing' AND j.slug = 'utah' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.utahlegalservices.org/node/7/bad-housing', 'Utah Legal Services', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'utah'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Before you move out, you must give your landlord a chance to fix the problems. If you do not do this, a judge may order you to pay some rent. A judge could make you pay rent for the remainder of the lease if the judge finds you didn’t have a good reason to move.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.utahlegalservices.org/node/7/bad-housing' AND j.slug = 'utah' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.virginialawhelp.org/en/breaking-your-lease-early', 'Virginia Law Help (Virginia Poverty Law Center)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'virginia'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you just leave, your landlord may be able to keep charging you rent and take you to court.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.virginialawhelp.org/en/breaking-your-lease-early' AND j.slug = 'virginia' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.virginialawhelp.org/en/can-i-end-my-lease-early-because-bad-conditions', 'Virginia Law Help (Virginia Poverty Law Center)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'virginia'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Even if you follow the steps correctly, your landlord can still take you to court for rent after you’ve moved and a judge will decide whether you had good enough reason to leave.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://www.virginialawhelp.org/en/can-i-end-my-lease-early-because-bad-conditions' AND j.slug = 'virginia' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://legalaidwv.org/legal-information/what-to-do-when-housing-conditions-are-bad-a-tenants-guide-to-safe-housing/', 'Legal Aid of West Virginia', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'west-virginia'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you move out early your landlord may try and sue you for breaching the lease. This is why is it is important to obtain a health department report to prove your housing was not ok.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://legalaidwv.org/legal-information/what-to-do-when-housing-conditions-are-bad-a-tenants-guide-to-safe-housing/' AND j.slug = 'west-virginia' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://legalaidwv.org/legal-information/what-to-do-when-housing-conditions-are-bad-a-tenants-guide-to-safe-housing/', 'Legal Aid of West Virginia', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'west-virginia'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Almost any instance of holding back rent will result in an eviction case being filed against you.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'https://legalaidwv.org/legal-information/what-to-do-when-housing-conditions-are-bad-a-tenants-guide-to-safe-housing/' AND j.slug = 'west-virginia' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'http://www.wislawhelp.org/page/423/how-end-tenancy-early', 'Wisconsin Judicare / WisLawHelp.org', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'wisconsin'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Other than the above, you generally can’t end a term tenancy early without remaining responsible for rent until the unit is re-rented. This is true even if you have very good reasons for early termination, such as buying a new house, or relocating for a new job or to take care of a sick family member.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'http://www.wislawhelp.org/page/423/how-end-tenancy-early' AND j.slug = 'wisconsin' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'http://www.wislawhelp.org/page/423/how-end-tenancy-early', 'Wisconsin Judicare / WisLawHelp.org', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'wisconsin'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'This is considered a form of constructive eviction, and is risky, because you are relying on a judge or court commissioner agreeing with your assessment of the situation.', 11
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-risk-owe' AND s.url = 'http://www.wislawhelp.org/page/423/how-end-tenancy-early' AND j.slug = 'wisconsin' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://public.courts.alaska.gov/web/forms/docs/pub-30.pdf', 'Alaska Court System (PUB-30)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'alaska'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If the tenant decides to move during the term of the lease, the tenant is usually still responsible for the rent for the rest of the lease period, unless the dwelling can be subleased or re-rented earlier.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://public.courts.alaska.gov/web/forms/docs/pub-30.pdf' AND j.slug = 'alaska' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://portal.ct.gov/-/media/cachm/magistratebook20185theditionfinalpdf.pdf?rev=f9311a640e5649eb99b803da20e281c9', 'Connecticut Judicial Branch (Housing Issues in the Small Claims Session)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'connecticut'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'When the tenant vacates voluntarily before the end of the lease term, without the landlord''s having terminated the lease, the landlord may be able to sue for the rent for the remaining period of the lease.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://portal.ct.gov/-/media/cachm/magistratebook20185theditionfinalpdf.pdf?rev=f9311a640e5649eb99b803da20e281c9' AND j.slug = 'connecticut' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://cca.hawaii.gov/wp-content/uploads/2026/02/2024-Landlord-Tenant-Handbook-Final.pdf', 'Hawaii Dept. of Commerce and Consumer Affairs, Office of Consumer Protection', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'hawaii'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If the tenant vacates the unit wrongfully, with no intention of resuming the tenancy, the landlord is entitled to the lesser of: A. The entire rent for the remainder of the term;', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://cca.hawaii.gov/wp-content/uploads/2026/02/2024-Landlord-Tenant-Handbook-Final.pdf' AND j.slug = 'hawaii' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.illinoislegalaid.org/legal-information/ending-lease', 'Illinois Legal Aid Online', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'illinois'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'the tenant must pay rent for the entire lease term even if they move out early. The landlord can sue the tenant for unpaid rent based on the lease.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://www.illinoislegalaid.org/legal-information/ending-lease' AND j.slug = 'illinois' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://iowalegalaid.org/resource/landlord-and-tenant-law-questions-and-answers/', 'Iowa Legal Aid', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'iowa'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you leave your lease early, you might be responsible for some or all of the rent for the rest of the term.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://iowalegalaid.org/resource/landlord-and-tenant-law-questions-and-answers/' AND j.slug = 'iowa' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://prd.webapps.chfs.ky.gov/kyrise/Content/docs/TenantsRightsManual.pdf', 'Legal Aid Society (Louisville), Tenants Rights Manual, hosted by Kentucky CHFS', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'kentucky'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you end your lease early, you may be liable for damages for unpaid rent.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://prd.webapps.chfs.ky.gov/kyrise/Content/docs/TenantsRightsManual.pdf' AND j.slug = 'kentucky' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://oag.maryland.gov/i-need-to/Documents/pdfs/landlordTenantPDF.pdf', 'Maryland Office of the Attorney General', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'maryland'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you break your lease, the landlord can hold you responsible for the rent due through the remainder of the lease.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://oag.maryland.gov/i-need-to/Documents/pdfs/landlordTenantPDF.pdf' AND j.slug = 'maryland' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.masslegalhelp.org/housing-apartments-shelter/tenants-rights/your-responsibilities-when-you-leave', 'Massachusetts Legal Help', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'massachusetts'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Your lease may also have a clause that says that if you leave before the end of your lease, you are responsible for the rent after you leave through the end of the lease term.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://www.masslegalhelp.org/housing-apartments-shelter/tenants-rights/your-responsibilities-when-you-leave' AND j.slug = 'massachusetts' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://montanalawhelp.org/resource/what-happens-if-i-break-my-lease-agreement-faq', 'Montana Legal Services Association (Montana LawHelp)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'montana'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'For example, if you move out at the end of the third month in a 12-month lease, the landlord could ask a court to make you pay for the rest of the 9 months on the lease, because you breached the contract.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://montanalawhelp.org/resource/what-happens-if-i-break-my-lease-agreement-faq' AND j.slug = 'montana' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://legalaidofnebraska.org/file_download/2595770f-9acb-4623-b5e2-c3b98d5fb01a', 'Legal Aid of Nebraska (Landlord and Tenant Handbook 2025)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'nebraska'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Even if you move out early, you are responsible for paying rent for the rest of the rental agreement period.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://legalaidofnebraska.org/file_download/2595770f-9acb-4623-b5e2-c3b98d5fb01a' AND j.slug = 'nebraska' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.lacsn.org/practice-areas/consumer-rights-project/tenant-rights/tips-for-moving-out-of-a-rental', 'Legal Aid Center of Southern Nevada', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'nevada'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you give notice of vacating during your lease term, you may be responsible for rent and/or early termination fees because you''re breaking your lease.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://www.lacsn.org/practice-areas/consumer-rights-project/tenant-rights/tips-for-moving-out-of-a-rental' AND j.slug = 'nevada' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.nj.gov/dca/codes/publications/pdf_lti/t_i_r.pdf', 'New Jersey Department of Community Affairs, Division of Housing and Community Resources', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'new-jersey'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If a tenant moves out before the end of the lease, the landlord may be able to hold the tenant responsible for the rent that becomes due until the premises is rented again, or until the lease ends, whichever occurs first.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://www.nj.gov/dca/codes/publications/pdf_lti/t_i_r.pdf' AND j.slug = 'new-jersey' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://ag.ny.gov/sites/default/files/tenants_rights.pdf', 'New York State Attorney General', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'new-york'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you leave your apartment or other rental home before your lease ends, your landlord must make a good-faith effort to fill the vacancy. If the landlord finds a new tenant and the new tenant’s rent is equal or higher to your rent, your lease is considered terminated and you are no longer liable for the rent.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://ag.ny.gov/sites/default/files/tenants_rights.pdf' AND j.slug = 'new-york' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.ohiolegalhelp.org/topic/move_out_early', 'Ohio Legal Help', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'ohio'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Usually no. In most cases, if you break your lease or move out early, you’re breaking the agreement between you and your landlord. You’ll probably be responsible for paying rent until the lease expires or until the landlord rents to someone else.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://www.ohiolegalhelp.org/topic/move_out_early' AND j.slug = 'ohio' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.okbar.org/landlord-rights/', 'Oklahoma Bar Association', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'oklahoma'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you try to re-rent and cannot find a tenant or find one who pays a lesser amount of rent, your original tenant will be liable to you for the amount of your loss.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://www.okbar.org/landlord-rights/' AND j.slug = 'oklahoma' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://oregonlawhelp.org/topics/housing/rental-housing/moving-or-moving-out/basic-information-about-moving-out', 'Oregon Law Help', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'oregon'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If your landlord doesn''t agree that you can move out early, they can require you to: Keep paying rent until they find a new renter; or Pay a fee for breaking your lease.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://oregonlawhelp.org/topics/housing/rental-housing/moving-or-moving-out/basic-information-about-moving-out' AND j.slug = 'oregon' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'http://www.rils.org/documents/tenant_rights.pdf', 'Rhode Island Legal Services', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'rhode-island'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you have a written lease and move out prior to the end of the term of the lease, your landlord may try to sue you for any lost rent he incurs during the term of the lease from the time of your departure until he finds a new tenant.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'http://www.rils.org/documents/tenant_rights.pdf' AND j.slug = 'rhode-island' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.tn.gov/content/dam/tn/health/documents/healthy-places/healthy-homes/HHW_LAS_TN_URLTA_Renters_Rights.pdf', 'Legal Aid Society (renter booklet hosted by the Tennessee Department of Health)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'tennessee'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Important! Don’t just break your lease and move out. Your landlord can take you to court. You may have to pay the rent even if you don’t live there now.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://www.tn.gov/content/dam/tn/health/documents/healthy-places/healthy-homes/HHW_LAS_TN_URLTA_Renters_Rights.pdf' AND j.slug = 'tennessee' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://vtlawhelp.org/ending-tenancy-and-moving-out', 'Vermont Legal Aid and Legal Services Vermont (VTLawHelp.org)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'vermont'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If it is your choice to leave, and your rental agreement is for a period of time, and it does not cover you leaving earlier, you have agreed to be responsible for the rent for all those months.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://vtlawhelp.org/ending-tenancy-and-moving-out' AND j.slug = 'vermont' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.washingtonlawhelp.org/en/tenants-rights-moving-out', 'Northwest Justice Project (WashingtonLawHelp.org)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'washington'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'But the landlord may still sue you for rent or other damages the landlord says you owe.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://www.washingtonlawhelp.org/en/tenants-rights-moving-out' AND j.slug = 'washington' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://ota.dc.gov/sites/default/files/dc/sites/ota/publication/attachments/Residential%20Lease%20Law%20FAQ-FINAL.pdf', 'DC Office of the Tenant Advocate', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'washington-dc'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'For example, a tenant who moves out early generally remains responsible for paying the rent through the end of the lease term.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://ota.dc.gov/sites/default/files/dc/sites/ota/publication/attachments/Residential%20Lease%20Law%20FAQ-FINAL.pdf' AND j.slug = 'washington-dc' AND s.kind IN ('gov_guidance', 'nonprofit');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.wyocourts.gov/legal-help-by-topic/requesting-repairs/', 'Wyoming Judicial Branch (court self-help)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'wyoming'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If you walk out on your lease without following the proper steps, your landlord may sue you for the rent remaining on your lease.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'leave-owe-short' AND s.url = 'https://www.wyocourts.gov/legal-help-by-topic/requesting-repairs/' AND j.slug = 'wyoming' AND s.kind IN ('gov_guidance', 'nonprofit');

