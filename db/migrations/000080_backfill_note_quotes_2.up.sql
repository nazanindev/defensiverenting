-- More backing quotes for the backfill page notes (ADR-016 A9, second batch,
-- 2026-10-12): award-win-and-pays for 22 more states, inspector-may-condemn
-- for 12. Not found: North Carolina and Tennessee for the award note. Not used:
-- nothing national exists for either note.
-- Each quote was found by a research agent, copied by script from the
-- checker's own dump of the live source, and read against the note it backs.
-- A source is tagged with its place (a state, or united-states for a national
-- page) and backs that place's pages. A row is inserted only when its source
-- passes the kind and place rules, so an odd existing row is skipped rather
-- than failing the deploy.

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://mobile.alacourt.gov/district-civil-and-small-claims/small-claims-faq/', 'Mobile County Circuit Court (Alabama)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'alabama'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'It is important to mention, it is easier to get a judgment than it is to collect on it. A judgment is not a guarantee of collection.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://mobile.alacourt.gov/district-civil-and-small-claims/small-claims-faq/' AND j.slug = 'alabama'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'It is important to mention, it is easier to get a judgment than it is to collect on it. A judgment is not a guarantee of collection.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://public.courts.alaska.gov/web/forms/docs/sc-100.pdf', 'Alaska Court System', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'alaska'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'All the court will give you is a judgment saying that you are entitled to collect the money from the defendant.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://public.courts.alaska.gov/web/forms/docs/sc-100.pdf' AND j.slug = 'alaska'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'All the court will give you is a judgment saying that you are entitled to collect the money from the defendant.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.justicecourts.maricopa.gov/media/lb4ho05z/howto_collect_money_jdgmt_cv.pdf', 'Maricopa County Justice Courts (Arizona)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'arizona'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'You may or may not collect. The Court provides a legal avenue to collect a judgment.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.justicecourts.maricopa.gov/media/lb4ho05z/howto_collect_money_jdgmt_cv.pdf' AND j.slug = 'arizona'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'You may or may not collect. The Court provides a legal avenue to collect a judgment.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://arkansasag.gov/divisions/public-protection/legal-resources/guide-to-small-claims-court/', 'Arkansas Attorney General', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'arkansas'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'You should be aware that the judge only determines who should prevail in the lawsuit; it is the prevailing party’s responsibility to make sure that the loser pays the amount the judge orders.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://arkansasag.gov/divisions/public-protection/legal-resources/guide-to-small-claims-court/' AND j.slug = 'arkansas'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'You should be aware that the judge only determines who should prevail in the lawsuit; it is the prevailing party’s responsibility to make sure that the loser pays the amount the judge orders.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.coloradojudicial.gov/self-help/cases-25000-or-less', 'Colorado Judicial Branch', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'colorado'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'The Court cannot collect your judgment for you.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.coloradojudicial.gov/self-help/cases-25000-or-less' AND j.slug = 'colorado'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'The Court cannot collect your judgment for you.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.jud.ct.gov/faq/smallclaims.html', 'Connecticut Judicial Branch', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'connecticut'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'The court does not collect the money for you, but you can request that the clerk issue an execution.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.jud.ct.gov/faq/smallclaims.html' AND j.slug = 'connecticut'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'The court does not collect the money for you, but you can request that the clerk issue an execution.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.nassauclerk.com/FAQ.aspx?QID=81', 'Nassau County Clerk of Court (Florida)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'florida'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'The court does not collect money damages for you.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.nassauclerk.com/FAQ.aspx?QID=81' AND j.slug = 'florida'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'The court does not collect money damages for you.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://consumer.georgia.gov/resolve-your-dispute/other-options-resolving-your-dispute', 'Georgia Governor''s Office of Consumer Protection', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'georgia'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'A judgment granting the plaintiff an award gives the plaintiff the right to collect the money damages from the defendant, but the plaintiff is responsible for actually collecting the award. The court cannot, and will not, collect awards for any party.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://consumer.georgia.gov/resolve-your-dispute/other-options-resolving-your-dispute' AND j.slug = 'georgia'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'A judgment granting the plaintiff an award gives the plaintiff the right to collect the money damages from the defendant, but the plaintiff is responsible for actually collecting the award. The court cannot, and will not, collect awards for any party.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.illinoislegalaid.org/legal-information/finding-legal-help-small-claims-case', 'Illinois Legal Aid Online', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'illinois'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Even if the plaintiff wins, that does not mean the defendant will pay the money owed. The defendant may be in a collection-proof status.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.illinoislegalaid.org/legal-information/finding-legal-help-small-claims-case' AND j.slug = 'illinois'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Even if the plaintiff wins, that does not mean the defendant will pay the money owed. The defendant may be in a collection-proof status.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.in.gov/courts/files/small-claims-manual.pdf', 'Indiana Office of Court Services', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'indiana'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Collecting the judgment is your responsibility. The length of time it will take to collect will depend upon both your diligence and the debtor’s ability to pay.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.in.gov/courts/files/small-claims-manual.pdf' AND j.slug = 'indiana'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Collecting the judgment is your responsibility. The length of time it will take to collect will depend upon both your diligence and the debtor’s ability to pay.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.kycourts.gov/Legal-Help/Documents/P-6_Small_Claims_Handbook_25.pdf', 'Kentucky Court of Justice', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'kentucky'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Although a judgment is legally enforceable, you are responsible for taking any additional action necessary to collect the money owed.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.kycourts.gov/Legal-Help/Documents/P-6_Small_Claims_Handbook_25.pdf' AND j.slug = 'kentucky'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Although a judgment is legally enforceable, you are responsible for taking any additional action necessary to collect the money owed.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.mdcourts.gov/legalhelp/smallclaims', 'Maryland Judiciary', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'maryland'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'The court will not collect the money owed to you. If the court ruled in your favor, and the other side does not pay you as ordered, you may need to take further steps to collect on the judgment.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.mdcourts.gov/legalhelp/smallclaims' AND j.slug = 'maryland'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'The court will not collect the money owed to you. If the court ruled in your favor, and the other side does not pay you as ordered, you may need to take further steps to collect on the judgment.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.mass.gov/info-details/small-claims-court', 'Massachusetts Trial Court (mass.gov)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'massachusetts'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Even if you win in the Small Claims Court, it may be difficult to collect a judgment from the defendant, especially if the losing party is not financially able to do so.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.mass.gov/info-details/small-claims-court' AND j.slug = 'massachusetts'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Even if you win in the Small Claims Court, it may be difficult to collect a judgment from the defendant, especially if the losing party is not financially able to do so.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://mncourts.gov/help-topics/conciliation-court', 'Minnesota Judicial Branch', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'minnesota'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'IMPORTANT: Winning a case in Conciliation Court does not guarantee payment. You are responsible for collecting your judgment from the other party. The court does not collect the payment for you.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://mncourts.gov/help-topics/conciliation-court' AND j.slug = 'minnesota'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'IMPORTANT: Winning a case in Conciliation Court does not guarantee payment. You are responsible for collecting your judgment from the other party. The court does not collect the payment for you.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://webstercountymo.gov/circuit-clerks-office/small-claims/', 'Webster County Circuit Clerk (Missouri)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'missouri'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'THE COURT IS NOT RESPONSIBLE FOR COLLECTING THE MONEY THAT THE JUDGE HAS AWARDED YOU.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://webstercountymo.gov/circuit-clerks-office/small-claims/' AND j.slug = 'missouri'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'THE COURT IS NOT RESPONSIBLE FOR COLLECTING THE MONEY THAT THE JUDGE HAS AWARDED YOU.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.njcourts.gov/self-help/collecting-money-civil', 'New Jersey Judiciary', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'new-jersey'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Although the court will try to help you collect the money owed to you, it cannot guarantee the debt will be paid.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.njcourts.gov/self-help/collecting-money-civil' AND j.slug = 'new-jersey'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Although the court will try to help you collect the money owed to you, it cannot guarantee the debt will be paid.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.ohiolegalhelp.org/topic/small-claims', 'Ohio Legal Help', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'ohio'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'After the court makes a judgment, you do not automatically get your money. You must take action to get the money.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.ohiolegalhelp.org/topic/small-claims' AND j.slug = 'ohio'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'After the court makes a judgment, you do not automatically get your money. You must take action to get the money.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://oregonlawhelp.org/topics/money-debt-and-consumer-issues/small-claims-court/what-do-after-your-small-claims-hearing', 'Oregon Law Help', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'oregon'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If the court rules in your favor, it''s up to you to collect the money or property. The court does not do this for you.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://oregonlawhelp.org/topics/money-debt-and-consumer-issues/small-claims-court/what-do-after-your-small-claims-hearing' AND j.slug = 'oregon'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'If the court rules in your favor, it''s up to you to collect the money or property. The court does not do this for you.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.lccpa.org/selfhelp/MDCourt/Topic1.nex', 'Lehigh County Court of Common Pleas (Pennsylvania)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'pennsylvania'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Remember, the court does not pay you and in most cases it is your responsibility to file the judgment and collect the money from the defendant.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.lccpa.org/selfhelp/MDCourt/Topic1.nex' AND j.slug = 'pennsylvania'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Remember, the court does not pay you and in most cases it is your responsibility to file the judgment and collect the money from the defendant.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.traviscountytx.gov/images/justices_of_peace/Doc/JP5/Civil_Forms/Filing_a_Lawsuit_in_Justice_Court_Information_Sheet_Eff_Jan_2026.pdf', 'Travis County Justice of the Peace, Precinct 5 (Texas)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'texas'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'This means that even though you might obtain a judgment against someone, it is likely you may never recover any money.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.traviscountytx.gov/images/justices_of_peace/Doc/JP5/Civil_Forms/Filing_a_Lawsuit_in_Justice_Court_Information_Sheet_Eff_Jan_2026.pdf' AND j.slug = 'texas'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'This means that even though you might obtain a judgment against someone, it is likely you may never recover any money.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://vacourts.gov/static/courts/gd/resources/manuals/gdman/gd_manual.pdf', 'Virginia General District Court Manual (Supreme Court of Virginia)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'virginia'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Execution of judgments in civil cases against the party who lost the case (judgment debtor) is the sole responsibility of the party in whose favor the judgment was granted (judgment creditor).', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://vacourts.gov/static/courts/gd/resources/manuals/gdman/gd_manual.pdf' AND j.slug = 'virginia'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Execution of judgments in civil cases against the party who lost the case (judgment debtor) is the sole responsibility of the party in whose favor the judgment was granted (judgment creditor).');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.wicourts.gov/formdisplay/SC-6090V_instructions.pdf?formNumber=SC-6090V&formType=Instructions&formatId=2&language=en', 'Wisconsin Court System', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'wisconsin'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'The court cannot do anything to automatically collect a judgment on your behalf and has no funds to pay your judgment for you. It is your responsibility to collect on your own judgment.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'award-win-and-pays' AND s.url = 'https://www.wicourts.gov/formdisplay/SC-6090V_instructions.pdf?formNumber=SC-6090V&formType=Instructions&formatId=2&language=en' AND j.slug = 'wisconsin'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'The court cannot do anything to automatically collect a judgment on your behalf and has no funds to pay your judgment for you. It is your responsibility to collect on your own judgment.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.santacruzca.gov/files/assets/city/v/1/pl/documents/code-compliance/tenant-housing-inspection-and-relocation-packet-2026.pdf', 'City of Santa Cruz Planning and Community Development', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'california'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'The City may post the unit as "substandard" and order it to be vacated. If the City orders a unit to be vacated, no one will be allowed to live there until the unit is repaired and made safe for habitation.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://www.santacruzca.gov/files/assets/city/v/1/pl/documents/code-compliance/tenant-housing-inspection-and-relocation-packet-2026.pdf' AND j.slug = 'california'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'The City may post the unit as "substandard" and order it to be vacated. If the City orders a unit to be vacated, no one will be allowed to live there until the unit is repaired and made safe for habitation.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.coloradolegalservices.org/housing/unsafe-housing-conditions/', 'Colorado Legal Services', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'colorado'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'However, if the conditions are very bad, the inspector may condemn the property. In that case, you would have to move.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://www.coloradolegalservices.org/housing/unsafe-housing-conditions/' AND j.slug = 'colorado'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'However, if the conditions are very bad, the inspector may condemn the property. In that case, you would have to move.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://miamidade.gov/building/standards/residential-minimum-housing.asp', 'Miami-Dade County Building (Minimum Housing Standards)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'florida'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Sometimes, houses and apartments become so rundown that they must be condemned. This means they are no longer fit to live in, and the occupants must move.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://miamidade.gov/building/standards/residential-minimum-housing.asp' AND j.slug = 'florida'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Sometimes, houses and apartments become so rundown that they must be condemned. This means they are no longer fit to live in, and the occupants must move.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://iowalegalaid.org/resource/how-do-i-get-my-landlord-to-make-repairs/', 'Iowa Legal Aid', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'iowa'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If the city inspector finds very bad housing problems, the inspector may say the rental unit is unfit to live in. In those cases, the tenants may have to move out right away.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://iowalegalaid.org/resource/how-do-i-get-my-landlord-to-make-repairs/' AND j.slug = 'iowa'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'If the city inspector finds very bad housing problems, the inspector may say the rental unit is unfit to live in. In those cases, the tenants may have to move out right away.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.peoples-law.org/baltimore-county-rental-and-housing-laws', 'People''s Law Library of Maryland (Baltimore County Code 35-5-206)', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'maryland'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'When the Code Official finds that a structure or part of a structure is unsafe or unfit for human use or occupancy, the official may order it: condemned; placarded (boarded up) and vacated. The structure or that part of the structure may not be reoccupied without approval of the Code Official.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://www.peoples-law.org/baltimore-county-rental-and-housing-laws' AND j.slug = 'maryland'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'When the Code Official finds that a structure or part of a structure is unsafe or unfit for human use or occupancy, the official may order it: condemned; placarded (boarded up) and vacated. The structure or that part of the structure may not be reoccupied without approval of the Code Official.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.masslegalhelp.org/housing-apartments-shelter/repairs-bad-conditions/what-if-your-building-condemned', 'Massachusetts Legal Help', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'massachusetts'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'This finding may lead to the Board of Health''s ordering that the building be condemned, that the landlord close up the property, and that the occupants vacate the portion that is unfit.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://www.masslegalhelp.org/housing-apartments-shelter/repairs-bad-conditions/what-if-your-building-condemned' AND j.slug = 'massachusetts'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'This finding may lead to the Board of Health''s ordering that the building be condemned, that the landlord close up the property, and that the occupants vacate the portion that is unfit.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.kcmo.gov/city-hall/departments/neighborhoods-housing-services/neighborhood-preservation/what-happens-when-you-receive-a-violation-letter', 'City of Kansas City, Missouri Neighborhoods and Housing Services', 'gov_guidance', j.id FROM jurisdictions j WHERE j.slug = 'missouri'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'In extreme circumstances, property maintenance violations may be so severe that the building is considered dangerous and uninhabitable (as explained in Section 56-532 of the Code of Ordinances); if this happens, an Order to Repair or Demolish will be sent to the property owner, and any residents will be ordered to leave the property for their own safety.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://www.kcmo.gov/city-hall/departments/neighborhoods-housing-services/neighborhood-preservation/what-happens-when-you-receive-a-violation-letter' AND j.slug = 'missouri'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'In extreme circumstances, property maintenance violations may be so severe that the building is considered dangerous and uninhabitable (as explained in Section 56-532 of the Code of Ordinances); if this happens, an Order to Repair or Demolish will be sent to the property owner, and any residents will be ordered to leave the property for their own safety.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://legalaidnc.org/resource/tenants-rights-evictions-repairs/', 'Legal Aid of North Carolina', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'north-carolina'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If your home is condemned by a city or county housing inspector, you may be required to vacate your home.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://legalaidnc.org/resource/tenants-rights-evictions-repairs/' AND j.slug = 'north-carolina'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'If your home is condemned by a city or county housing inspector, you may be required to vacate your home.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://cohhio.org/support/tenant-rights/repairs-and-living-conditions-tenants/', 'Coalition on Homelessness and Housing in Ohio (COHHIO)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'ohio'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'Sometimes local agencies can be very helpful getting landlords to make repairs. But other times agencies will just condemn a home with big problems and make the tenants homeless.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://cohhio.org/support/tenant-rights/repairs-and-living-conditions-tenants/' AND j.slug = 'ohio'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'Sometimes local agencies can be very helpful getting landlords to make repairs. But other times agencies will just condemn a home with big problems and make the tenants homeless.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://oregonlawhelp.org/media/168/download?inline=', 'Oregon Law Help (tenant guide)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'oregon'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'But there are risks with calling an inspector while you are still living in the unit. The inspector could also force you to move if the unit is very dangerous.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://oregonlawhelp.org/media/168/download?inline=' AND j.slug = 'oregon'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'But there are risks with calling an inspector while you are still living in the unit. The inspector could also force you to move if the unit is very dangerous.');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://www.palawhelp.org/resource/housing-code-fact-sheet', 'PALawHELP.org (Housing Code Fact Sheet)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'pennsylvania'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If the problems indicated in the housing code inspection are very severe and cannot be repaired quickly, the Code Enforcement Officer may condemn the unit. Condemnation is required when habitability problems are very severe, posing a threat to health and safety. In that situation, you would have to find alternate housing with very little advance notice, so you make want to consider this possibility before calling the Code Enforcement Officer', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://www.palawhelp.org/resource/housing-code-fact-sheet' AND j.slug = 'pennsylvania'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'If the problems indicated in the housing code inspection are very severe and cannot be repaired quickly, the Code Enforcement Officer may condemn the unit. Condemnation is required when habitability problems are very severe, posing a threat to health and safety. In that situation, you would have to find alternate housing with very little advance notice, so you make want to consider this possibility before calling the Code Enforcement Officer');

INSERT INTO sources (url, publisher, kind, jurisdiction_id)
SELECT 'https://legalaction.org/wp-content/uploads/2024/01/Tenant-Sourcebook.pdf', 'Legal Action of Wisconsin (Tenant Sourcebook)', 'nonprofit', j.id FROM jurisdictions j WHERE j.slug = 'wisconsin'
ON CONFLICT (url) DO UPDATE SET jurisdiction_id = COALESCE(sources.jurisdiction_id, EXCLUDED.jurisdiction_id);
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id, 'If the apartment has been posted or “red-tagged” by the Fire Department or Housing Inspector, you should move out as soon as possible.', 10
FROM advice a, sources s JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE a.slug = 'inspector-may-condemn' AND s.url = 'https://legalaction.org/wp-content/uploads/2024/01/Tenant-Sourcebook.pdf' AND j.slug = 'wisconsin'
  AND s.kind IN ('gov_guidance', 'nonprofit')
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id AND c.source_id = s.id AND c.quote = 'If the apartment has been posted or “red-tagged” by the Fire Department or Housing Inspector, you should move out as soon as possible.');

