-- A plain habit that says nothing about the law or what will happen needs
-- no source (Nazanin, 2026-10-09: "we dont need a source for read your
-- lease"). Such a tip is site voice, like the disclaimer. Anything that
-- states a consequence or a legal fact still needs a quote. Entries are
-- written only here, in migrations a person reviews, which is where that
-- line is held. A site-voice tip still shows a source chip where the
-- page's place has one.
DO $$
DECLARE c TEXT;
BEGIN
    SELECT conname INTO c FROM pg_constraint
    WHERE conrelid = 'advice'::regclass AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%site_voice%';
    IF c IS NOT NULL THEN
        EXECUTE format('ALTER TABLE advice DROP CONSTRAINT %I', c);
    END IF;
END $$;

-- A note that warns states a consequence; it is never site voice.
ALTER TABLE advice ADD CONSTRAINT advice_warning_needs_source CHECK (NOT (site_voice AND warns IS NOT NULL));

UPDATE advice SET site_voice = true
WHERE slug IN ('read-your-lease', 'write-down-the-problem', 'photos-when-you-leave');
