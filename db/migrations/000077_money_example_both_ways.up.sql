-- The money example covers amounts a renter can win too, not only what a
-- landlord can collect (Nazanin 2026-10-11), so it no longer names who
-- sues. The CFPB quote backed only the landlord side; the line is now site
-- voice, explaining that an example is an example.
UPDATE advice
SET body_md = 'This is only an example to show how the rule works. If it goes to court, a court decides the real amount.',
    site_voice = true
WHERE slug = 'money-example';
DELETE FROM advice_citations WHERE advice_id = (SELECT id FROM advice WHERE slug = 'money-example');
