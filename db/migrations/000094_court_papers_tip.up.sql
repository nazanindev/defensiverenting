-- The court papers tip said "Go to court". In states where the renter must
-- file a written answer first, that is the wrong step. The new wording names
-- no step; the statement above the tip does (ADR-035 D8).
UPDATE advice
SET body_md = 'If you get court papers, do not ignore them. If you do not respond by the date on the papers, you can lose the case.'
WHERE slug = 'answer-court-papers';
