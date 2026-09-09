-- A durable identity for a claim across edits (ADR-014 D1).
--
-- Every save re-links a playbook to a fresh set of statements rows and drops
-- the orphans, so statements.id changes on each save — including the
-- once-a-minute autosave. Nothing outside a single save can point at a
-- statement and still be pointing at it a minute later. The key is the value
-- that stays: the edit form round-trips it, the save paths carry it onto the
-- replacement row, and a draft revision keeps the keys of the statements it
-- kept. A live page and its draft revision may therefore share a key, which
-- is why the index is not unique.
ALTER TABLE statements ADD COLUMN key UUID NOT NULL DEFAULT gen_random_uuid();
CREATE INDEX statements_key_idx ON statements (key);
