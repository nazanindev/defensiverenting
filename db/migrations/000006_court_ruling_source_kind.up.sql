ALTER TABLE sources DROP CONSTRAINT sources_kind_check;
ALTER TABLE sources ADD CONSTRAINT sources_kind_check
    CHECK (kind IN ('statute', 'regulation', 'gov_guidance', 'nonprofit', 'editorial', 'court_ruling'));
