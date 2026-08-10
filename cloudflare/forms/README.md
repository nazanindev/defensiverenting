# Form intake Worker

Receives the `/report` and `/contact` forms from renterlaw.org, stores each
submission in D1, and emails a notification through Email Routing.

## Why this is not in the Go server

The Go app already has Postgres and could take a form POST in about a hundred
lines. It lives here instead for three reasons:

- **Spam never reaches the origin.** Turnstile and the honeypot are evaluated
  at Cloudflare's edge, so a flood costs the Fly machine nothing.
- **Submissions survive the site being down.** A reader reporting that a legal
  aid office has closed is telling us something we cannot recover later.
- **Reader-submitted text stays out of the content database.** The Postgres
  schema enforces the citation invariant. Unverified free text has no place in
  it.

The cost is a second system to deploy and a second place to look. If that
stops being worth it, the whole thing is one `fetch` handler to port.

## How a submission flows

```
renterlaw.org/report                Go template, site CSS
    │  POST (plain HTML form, no JS required)
    ▼
forms.renterlaw.org/report          this Worker
    │  honeypot → field shape → Turnstile → rate limit → D1 → email
    ▼
renterlaw.org/thanks                303, so a reload cannot resend
```

Rejections redirect to `renterlaw.org/report?error=<code>`. The Go handler
(`internal/http/handlers/forms.go`) turns the code into a sentence; the codes
are the entire contract between the two services.

## First-time setup

Everything below is on Cloudflare's free tier. Run the commands from this
directory.

### 1. Turnstile keys

Dashboard → **Turnstile** → **Add widget**.

- Widget name: `renterlaw forms`
- Hostnames: `renterlaw.org`, `www.renterlaw.org`
- Widget mode: **Managed**

You get two keys. The **site key** is public and goes in `fly.toml`. The
**secret key** goes into the Worker in step 4.

For a staging environment, Cloudflare publishes dummy keys that always pass:
site key `1x00000000000000000000AA`, secret `1x0000000000000000000000000000000AA`.

### 2. D1 database

```sh
npx wrangler d1 create renterlaw-forms
```

Copy the `database_id` it prints into `wrangler.toml`, replacing
`REPLACE_WITH_ID_FROM_D1_CREATE`. Then create the table:

```sh
npx wrangler d1 execute renterlaw-forms --remote --file=./schema.sql
```

`--remote` matters. Without it you get a local SQLite file and the deployed
Worker writes into an empty database.

### 3. Email Routing

Email Routing is already enabled on renterlaw.org (the MX records point at
`route*.mx.cloudflare.net`). Two things to confirm under **Email** → **Email
Routing**:

- **Destination addresses**: the address in `wrangler.toml` must be listed and
  verified. Cloudflare refuses to send anywhere else, which is what makes this
  binding safe to expose to a public form.
- **Routing rules**: add `forms@renterlaw.org` → your inbox. Sending does not
  require it, but bounces and stray replies go to that address, and without a
  rule they are dropped.

### 4. Secrets

```sh
npx wrangler secret put TURNSTILE_SECRET_KEY   # from step 1
npx wrangler secret put IP_SALT                # openssl rand -hex 32
```

`IP_SALT` salts the hash used for rate limiting. Rotating it resets everyone's
hourly count, which is harmless. Raw IP addresses are never stored: see the
comment at the top of `schema.sql`.

### 5. Deploy the Worker

```sh
npx wrangler deploy
```

`custom_domain = true` in `wrangler.toml` creates the `forms.renterlaw.org`
DNS record for you. The public site's own record is untouched and still points
straight at Fly, so nothing about renterlaw.org's routing changes.

### 6. Point the site at it

In `fly.toml`, replace `REPLACE_WITH_TURNSTILE_SITE_KEY` with the site key from
step 1, then deploy:

```sh
fly deploy
```

`FORMS_URL` is already set there. Both values are public, which is why they are
`[env]` entries rather than `fly secrets`.

### 7. Check it end to end

Submit the real form at <https://renterlaw.org/report>, then:

```sh
npx wrangler d1 execute renterlaw-forms --remote \
  --command "SELECT created_at, kind, problem, page_url FROM submissions ORDER BY created_at DESC LIMIT 5"
```

The row should be there and the email should have arrived. If either is
missing, `npx wrangler tail` streams live logs; every failure path logs before
it redirects.

## Reading submissions

Newest first:

```sh
npx wrangler d1 execute renterlaw-forms --remote \
  --command "SELECT created_at, kind, problem, page_url, org_name, email, message
               FROM submissions WHERE status = 'new' ORDER BY created_at DESC"
```

Mark one handled (the ID is in every notification email):

```sh
npx wrangler d1 execute renterlaw-forms --remote \
  --command "UPDATE submissions SET status = 'resolved' WHERE id = '<id>'"
```

Pages people report most, which is the useful signal once there is volume:

```sh
npx wrangler d1 execute renterlaw-forms --remote \
  --command "SELECT page_url, COUNT(*) AS n FROM submissions
              WHERE kind = 'report' GROUP BY page_url ORDER BY n DESC LIMIT 20"
```

## Tests

```sh
npm test          # node --test, no dependencies to install
```

Covers the two pieces that fail quietly: `sameSiteURL`, which decides what a
stranger can put in the database, and the message builder, whose bugs show up
as notifications simply not arriving. The Go side has its own tests for the
form pages in `internal/http/handlers/forms_test.go` and
`web/templates/reportlink_test.go`.

## Local development

Leave `TURNSTILE_SITE_KEY` unset and the widget does not render, which is what
you want: nothing verifies the token locally.

To exercise the Worker itself, run it alongside the Go server and point the
site at it:

```sh
npx wrangler dev --local          # serves http://localhost:8787
FORMS_URL=http://localhost:8787 go run ./cmd/server
```

With `--local` the D1 writes go to a local SQLite file and `env.NOTIFY.send`
is not available, so the email step logs a failure and the submission still
stores. That is the same behaviour as production when mail fails.

## Spam handling, in order of cost

1. **Honeypot** (`company` field). Hidden from readers and screen readers.
   Anything that fills it gets the success page, so a bot has no signal to
   retry with the field cleared. Costs nothing.
2. **Field shape.** Length and email format, before any subrequest.
3. **Turnstile.** One subrequest, only for submissions that already look real.
4. **Rate limit.** 5 per hour per salted IP hash, checked in D1.

If spam still gets through, the next step is a Cloudflare rate-limiting rule on
`forms.renterlaw.org` rather than more logic here.
