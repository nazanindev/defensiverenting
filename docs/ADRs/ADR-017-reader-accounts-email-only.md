# ADR-017: Reader accounts are a magic link and an email address, and the first thing they hold is a location

| | |
|---|---|
| Status | Accepted |
| Date | 2026-09-11 |
| Amends | DESIGN.md §11 (the auth seam) and §15 (v2.5 sequencing) |

## Context

The site has a location picker. The choice lives in localStorage, so it holds on one device and vanishes on the next. DESIGN.md has said since the start that accounts come later, as a magic link, and that a logged-in renter is the end state. The first real reader wrote in on 2026-09-11, and the direction is a paid tier that funds per-renter work. Both need an account to hang off. Neither needs a profile.

Two constraints already in the codebase shape the answer. Browse HTML is served with a shared public cache, so nothing about who is reading may be rendered into it (ADR-006, the scope-script comment in layout.html). And the forms Worker records as little as it can about a sender, hashing the IP rather than storing it, because people write about their own housing here and a table of who-said-what is a liability with no use.

The question this ADR settles is how much to know about a signed-in reader. The answer is: the least that makes sign-in work.

## Decision

### D1. Sign-in is a link sent to an email address

A reader types an email address and receives a link that works once, for fifteen minutes. Opening it sets a session cookie good for ninety days. There is no password, no username, no sign-up step apart from the first link. The user row is created when a link is used, never when one is requested, so typing a stranger's address creates nothing for them.

Passkeys were considered and rejected for now: no recovery path, and cross-device only works when the platform syncs keys. Third-party sign-in was rejected because it hands over a name and a profile nobody asked for, and ties a housing account to a big provider.

### D2. The account stores an email address and a location, and nothing else

```
users        id, email, created_at
login_tokens token_hash, email, expires_at, used_at, created_at
sessions     token_hash, user_id, expires_at, created_at
user_prefs   user_id, location_id -> jurisdictions, updated_at
```

No name. No password hash. No reading history. Tokens are stored as SHA-256 hashes, so a copy of the table signs no one in. The location is a foreign key to the jurisdiction row, so a renamed slug follows and a deleted place clears. Adding a preference is adding a column to `user_prefs`; adding a kind of thing a reader owns is a new table keyed by `user_id`.

### D3. Signed-in state reaches the page only through uncached JSON

Every account route sends `Cache-Control: private, no-store`. Browse pages stay byte-identical for every reader. The header says "Sign in" in the HTML; a script flips it to "Account" when a second, secret-free cookie (`rl_in`) says there is a session. The scope script asks `/api/me` after load, only when that cookie is present, and writes changes with `PUT /api/me/location`.

Sync rule: the server's location wins over the device's when both are set; a fresh account adopts whatever the device already remembered; a change is written to both. Signed out, the picker works exactly as before this ADR.

### D4. Mail goes through Resend, from the server

The forms Worker cannot send to arbitrary addresses by design, so the server sends the link itself with one HTTPS POST to Resend. `RESEND_API_KEY` unset means links are written to the log instead, which is what development wants and what production must never run with. `MAIL_FROM` names the sender; its domain must be verified in Resend.

### D5. The form is rate limited and same-origin only

A sign-in form is a way to make the server mail anyone. Two caps: five links per address per hour, counted in the database, and twenty per client per hour, counted in memory. A capped request gets the same "check your email" page as a real one, so the form never says which addresses exist. State-changing routes reject cross-site requests by `Sec-Fetch-Site` and `Origin`; the location endpoint also requires a JSON body, which a cross-site form cannot send.

## Consequences

- A reader who signs in on a phone and a laptop sees the same location on both. That is the whole feature today.
- The header carries one more link on every page. It is static text and costs the cache nothing.
- Production needs `RESEND_API_KEY` set as a Fly secret before this deploys, or readers will be told to check an inbox nothing arrives in.
- The account is the seam the paid tier and the watchlist will hang off. Neither is in this ADR; each gets its own, and each adds a table rather than a column on `users`.
- `internal/http/middleware/auth.go` as DESIGN.md §11 described it was never written and now will not be. The session resolves inside the account handler, and browse handlers still know nothing about readers, which is the property that section was protecting.
