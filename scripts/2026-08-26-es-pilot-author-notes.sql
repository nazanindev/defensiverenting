-- Attach the 2026-08-26 Spanish-pilot review notes to each affected playbook's
-- author_notes, so the editor sees them on the article instead of in a chat
-- log. 14 Spanish drafts get their translation-review notes; 2 published
-- English pages get notes about defects the translation agents found in them.
--
-- Appends (never overwrites): existing notes are kept and the new note is
-- added after a blank line. Run ONCE; a second run appends duplicates.
-- Targets are keyed (jurisdiction slug, topic slug, language, status), which
-- is unique per migration 000015. The final SELECT lists every row touched;
-- expect 16.
BEGIN;

WITH v(jslug, tslug, lang, pstatus, note) AS (VALUES
  ('austin', 'cant-pay-rent', 'es', 'draft', $n$[AI pilot 2026-08-26] Statement 6 keeps two time facts from the English claim: the 10 to 21 day hearing range as background, and the 5 day appeal deadline. The lint accepted it; confirm the reading. Worked examples added: 3 x $50 = $150 for the illegal-fee penalty, and $1,000 + $1,000 = $2,000 for the lockout example. Citation kinds inferred (get_playbook returns none); no concept tags set.$n$),
  ('austin', 'eviction-defense', 'es', 'draft', $n$[AI pilot 2026-08-26] Two clock-start additions are sourced but absent from the English body: the 3 day notice counted from delivery (Tex. Prop. Code § 24.005(g)) and the 5 day appeal counted from the day the judge signs (TexasLawHelp). Flag if exact parity with English is preferred. The appeal statement carries two 5 day deadlines with ordering words, mirroring English. Official document names kept in English with Spanish glosses. Citation kinds inferred; no concept tags set.$n$),
  ('austin', 'repairs-and-habitability', 'es', 'draft', $n$[AI pilot 2026-08-26] Title avoids "habitabilidad": "Reparaciones y vivienda segura en Austin"; change if you prefer another. The $1,200 rent in the worked dollar examples is illustrative, not taken from the English page. Citation kinds inferred; no concept tags set.$n$),
  ('austin', 'security-deposits', 'es', 'draft', $n$[AI pilot 2026-08-26] Worked example added for the 3x penalty (3 veces una renta de $1,000 son $3,000); plain-word glosses added for "vivienda pública o subsidiada" and "demandar". Citation kinds inferred; no concept tags set.$n$),
  ('austin', 'landlord-entry', 'es', 'draft', $n$[AI pilot 2026-08-26] The English source page has a duplicated emergency-entry sentence (translated once here) and a garbled "The common Texas Apartment Association" phrase, likely missing "lease"; see the note on the English page. Worked example added for the lockout penalty (1 mes de una renta de $1,000 más $1,000 da $2,000). Citation kinds inferred; no concept tags set.$n$),
  ('austin', 'rent-increase', 'es', 'draft', $n$[AI pilot 2026-08-26] Statement 13 adds "desde el día en que le entregan el aviso" to satisfy the clock rule; § 24.005(g) supports it but the citation quote remains § 24.005(a). Confirm you are comfortable with that. Terminology: "el dueño" for landlord, "contrato de renta" for lease, "tope de renta" for rent cap. Citation kinds inferred; no concept tags set.$n$),
  ('austin', 'resource-directory', 'es', 'draft', $n$[AI pilot 2026-08-26] Fresh research, not a translation: 2 organisations shared with the English page, 7 new Spanish-serving ones. Before publish, confirm: the VLS Spanish clinic page is stale COVID-era content (only structural facts quoted); El Buen Samaritano's monthly application window and 512-522-1097; "I Belong in Austin" is stated as closed since March 2026; the 211 "free, 24 hours" claim is quoted from austintexas.gov/rent rather than 211texas.org; BASTA and Catholic Charities entries carry no phone because their fetched pages publish none. Citation kinds chosen by the agent; no concept tags set.$n$),
  ('new-york-city', 'cant-pay-rent', 'es', 'draft', $n$[AI pilot 2026-08-26] access.nyc.gov (One Shot Deal, Homebase) was reachable only via an archive snapshot; confirm the live pages still match. Statement 2 adds the clock start "desde que usted lo recibe", implicit in the English. The multi-line Homebase quote was submitted with this fetch's whitespace (identical words, different line endings than the stored English quote). Citation kinds inferred; no concept tags set.$n$),
  ('new-york-city', 'eviction-defense', 'es', 'draft', $n$[AI pilot 2026-08-26] Statement 4 adds a clock-start sentence for the 14 day deadline; statement 11 adds the worked example "3 veces un daño de $1,000 son $3,000". Statement 5 keeps the 30/60/90 day notice periods bundled, mirroring English; the rent-increase pages split them into case statements, consider matching. One Shot Deal page verified via archive snapshot. Citation kinds inferred; no concept tags set.$n$),
  ('new-york-city', 'repairs-and-habitability', 'es', 'draft', $n$[AI pilot 2026-08-26] Fahrenheit made explicit on the 55/68/62/120 degree figures (the HPD source says Fahrenheit; the English page says only "degrees"). One English sentence was split in two for the sentence-length rule, no claim change. One Shot Deal page verified via archive snapshot; confirm live. Citation kinds inferred; no concept tags set.$n$),
  ('new-york-city', 'security-deposits', 'es', 'draft', $n$[AI pilot 2026-08-26] The HPD quote preserves the source's "retianed" typo on purpose; do not fix it inside the quote. Worked example added: "2 veces un depósito de $1,000 son $2,000" (the English has none). access.nyc.gov verified via archive snapshot. Citation kinds inferred; no concept tags set.$n$),
  ('new-york-city', 'landlord-entry', 'es', 'draft', $n$[AI pilot 2026-08-26] Worked example added for treble damages ("si sus daños son $1,000, el pago puede ser $3,000") and a clock-start for the 30 day occupancy threshold. The AG page renders "RPAPL 768" with a non-breaking space while the stored English quote has a plain double space; if a quote diff ever looks off by one space, this is why. Citation kinds inferred; no concept tags set.$n$),
  ('new-york-city', 'rent-increase', 'es', 'draft', $n$[AI pilot 2026-08-26] 17 statements vs 14 in English: the deadline-pile lint split the bundled 30/60/90 day notice statement into a lead statement plus 3 case statements, adding verbatim RPL § 226-c(2)(b), (2)(c), (2)(d) quotes from nysenate.gov. Spot-check those 3 new citations and confirm the split; the English page has the matching note. Time-sensitive figures carried from English: Good Cause 8.38%, RGB Order #57 caps, Order #58 freeze. Citation kinds inferred; no concept tags set.$n$),
  ('new-york-city', 'resource-directory', 'es', 'draft', $n$[AI pilot 2026-08-26] Fresh research, not a translation: 2 organisations shared with the English page, 9 new. Before publish, confirm: NMIC's "intake suspendida hasta julio de 2026" note has lapsed, so the draft gives only the general 212-822-8300 number; Met Council's hotline page is dated December 2024, spot-check hours; the One Shot Deal Spanish page was verified via archive snapshot; ACCESS NYC's tenant-legal-services page 403'd, so Right to Counsel cites nyc.gov instead. No "answers in Spanish" claim was made for Met Council or Make the Road because their fetched pages do not say so. Citation kinds chosen by the agent; no concept tags set.$n$),
  ('austin', 'landlord-entry', 'en', 'published', $n$[AI pilot 2026-08-26] Found while translating: statement 2 repeats the sentence pair "Emergencies are different. In an emergency, the landlord can enter with no advance notice to protect you and the property." twice; statement 1 says "The common Texas Apartment Association also lists many entry reasons", likely missing "lease". Both need an edit.$n$),
  ('new-york-city', 'rent-increase', 'en', 'published', $n$[AI pilot 2026-08-26] The statement bundling the 30/60/90 day notice periods predates the deadline-pile voice rule and would fail today's lint. The Spanish draft splits it into a lead statement plus 3 case statements citing RPL § 226-c(2)(b)-(d); consider backporting that split here.$n$)
),
hit AS (
  UPDATE playbooks pb
  SET author_notes = CASE WHEN pb.author_notes = '' THEN v.note
                          ELSE pb.author_notes || E'\n\n' || v.note END
  FROM v
  JOIN jurisdictions j ON j.slug = v.jslug
  JOIN topics t ON t.slug = v.tslug
  WHERE pb.jurisdiction_id = j.id
    AND pb.topic_id = t.id
    AND pb.language = v.lang
    AND pb.status = v.pstatus
  RETURNING v.jslug, v.tslug, v.lang, v.pstatus
)
SELECT jslug, tslug, lang, pstatus FROM hit ORDER BY jslug, tslug, lang;

COMMIT;
