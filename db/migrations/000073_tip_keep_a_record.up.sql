-- A plain habit, so site voice (000068): it says to keep the record and
-- promises nothing about what a court makes of it. Whether a text or email
-- counts as notice is a legal claim and goes on a state's page as a cited
-- statement where that state's source says so (Nazanin, 2026-10-11).
INSERT INTO advice (slug, kind, site_voice, body_md) VALUES
    ('keep-a-record', 'tip', true,
     'Keep a copy of every letter you give your landlord, and keep your mail receipt. Save the texts and emails you send your landlord, and any replies.')
ON CONFLICT (slug) DO NOTHING;
