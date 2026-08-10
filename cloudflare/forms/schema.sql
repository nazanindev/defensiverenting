-- Submissions from the public report and contact forms.
--
-- Raw IP addresses are deliberately absent. Rate limiting needs to recognise a
-- repeat sender, not identify one, so only a salted hash is stored and the salt
-- lives in a Worker secret. People report problems with their own housing here,
-- and a table of who-said-what-from-where is a liability we have no use for.
CREATE TABLE IF NOT EXISTS submissions (
  id         TEXT PRIMARY KEY,
  kind       TEXT NOT NULL CHECK (kind IN ('report', 'contact')),
  created_at TEXT NOT NULL,
  status     TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'triaged', 'resolved', 'spam')),

  -- report only: which page and which organisation the reader was looking at
  page_url   TEXT,
  org_name   TEXT,
  problem    TEXT,

  name       TEXT,
  email      TEXT,
  message    TEXT NOT NULL,

  ip_hash    TEXT NOT NULL,
  country    TEXT,
  user_agent TEXT
);

-- Triage queue: newest unhandled submissions first.
CREATE INDEX IF NOT EXISTS submissions_status_created
  ON submissions (status, created_at DESC);

-- Rate limit lookup: recent submissions from one sender.
CREATE INDEX IF NOT EXISTS submissions_ip_created
  ON submissions (ip_hash, created_at DESC);

-- "What is wrong with this page" across all reports.
CREATE INDEX IF NOT EXISTS submissions_page
  ON submissions (page_url) WHERE page_url IS NOT NULL;
