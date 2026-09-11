-- A confirmation records how it was made, not only when. Every path that
-- stamps checked_at (the drafting guardrail, the authoring form's verifier, a
-- check-sources run, the queue's approval) now writes the tier and extractor
-- that produced the text the quote was found in, a hash of that text, and the
-- passage around the quote. A later check compares against this baseline: an
-- equal hash is "unchanged" with no matching needed, a differing hash with the
-- quote missing is drift shown as old passage beside new, and a quote pdftotext
-- confirmed cannot be declared missing by the pure-Go PDF reader. Existing
-- rows keep empty strings: nothing was recorded for them, and an empty
-- baseline is compared the old way, by matching the quote alone.
ALTER TABLE citations
  ADD COLUMN checked_via       TEXT NOT NULL DEFAULT '',  -- direct | render; '' for attestations and pre-receipt rows
  ADD COLUMN checked_extractor TEXT NOT NULL DEFAULT '',  -- html | render | pdftotext | pdfgo
  ADD COLUMN checked_hash      TEXT NOT NULL DEFAULT '',  -- sha256 of the whitespace-normalized source text
  ADD COLUMN checked_context   TEXT NOT NULL DEFAULT '';  -- the passage around the quote in that text

-- last_checked_at keeps its meaning: the checker read this source and examined
-- its quotes. A run that reached the source but got no readable text (a script
-- shell, a bot-check page) used to stamp it anyway and file every quote as
-- drift. Now such a run records only that it tried, and why it could not
-- examine anything, so the dashboard can say "unreadable from the authoring
-- server since Tuesday" instead of nothing.
ALTER TABLE sources
  ADD COLUMN last_fetch_at   TIMESTAMPTZ,
  ADD COLUMN last_fetch_note TEXT NOT NULL DEFAULT '';
