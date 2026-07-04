-- Store the verbatim source line backing each citation. The AI-assisted drafting
-- pipeline pre-fills this with the exact text pulled from the fetched source, and
-- the save step rejects any quote that is not a literal substring of that source,
-- so a citation can never carry a fabricated quote. locator still holds the
-- section pointer (e.g. "§ 15B"); quote holds the words themselves.
ALTER TABLE citations ADD COLUMN quote TEXT NOT NULL DEFAULT '';
