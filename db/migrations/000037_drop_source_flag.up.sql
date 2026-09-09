-- The source-level flag is retired (ADR-014 D4). A cited quote that no
-- longer appears at its source now files a source-drift proposal against the
-- statement that cites it, with the nearest passage from the new fetch as
-- evidence, and the reviewer's dismissal is a rejection with a note. The flag
-- carried none of that: it named a URL and asked the author to go looking.
ALTER TABLE sources DROP COLUMN IF EXISTS flagged_at;
