-- A reviewer can override the automated quote check when a source blocks
-- fetching outright (see quoteVerifier in cmd/authoring/main.go): they attest
-- they read the source themselves. manually_verified marks that citation as
-- human-attested rather than machine-matched, so the two stay distinguishable
-- instead of quietly blurring together.
ALTER TABLE citations ADD COLUMN manually_verified BOOLEAN NOT NULL DEFAULT false;
