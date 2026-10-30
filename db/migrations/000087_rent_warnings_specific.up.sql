-- The rent warnings say what actually happens (Nazanin, 2026-10-29): "we
-- should be specific, say you can owe the rent if the landlord sues you for
-- it and wins." Nothing is automatic. The landlord has to go to court and
-- win: an eviction case for unpaid rent, or a suit for the rent left on the
-- lease. "If the judge rules for your landlord" says that a judge decides,
-- so the separate "A court decides." note is not needed on these pages.
--
-- withhold-risk-evict is on draft pages only, so it changes in place. The
-- moving-out note is a new entry: leave-owe-short and leave-court-may-disagree
-- are on published pages and on other topics, and live pages stay human.
UPDATE advice SET
  lead = 'Your landlord can take you to court.',
  body_md = 'If you hold back rent, pay less, or take repair costs out of your rent, your landlord can file an eviction case for unpaid rent. If the judge rules for your landlord, you can owe the rent and court costs and have to move out. Your proof matters. Even a case you win can show up when you apply for a new place. Get legal help first.'
WHERE slug = 'withhold-risk-evict';

INSERT INTO advice (slug, kind, warns, site_voice, lead, body_md) VALUES (
  'leave-repairs-sue-rent', 'page_note', 'owe', true,
  'Your landlord can sue you for the rent.',
  'If you move out early because of repairs, your landlord can sue you for the rent up to the end of your lease. If the judge rules for your landlord, you can owe that rent. Your proof matters. Get legal help first.'
);
