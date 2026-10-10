UPDATE advice SET site_voice = false
WHERE slug IN ('read-your-lease', 'write-down-the-problem', 'photos-when-you-leave');
ALTER TABLE advice DROP CONSTRAINT IF EXISTS advice_warning_needs_source;
ALTER TABLE advice ADD CHECK (NOT site_voice OR kind = 'page_note');
