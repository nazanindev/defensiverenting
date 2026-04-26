# Defensive Renting

Citation-first tenant-rights playbooks in Go + Postgres.

## Current Migration Status

- Boston topics: migrated to markdown in `content/boston/*.en.md`
- Seattle topics: still legacy HTML in `seattle/*.html` (next migration batch)
- Legacy static pages are still in the repo until routing/deploy cutover is complete

## How Legacy HTML Migration Works

For each legacy topic page (example: `boston/heat-not-working.html`):

1. Create/update the markdown playbook at `content/<city>/<topic>.en.md`
2. Move legal guidance into atomic statements
3. Ensure every statement ends with at least one citation token (`[source-id]` or `[source-id:locator]`)
4. Run ingest to load content into Postgres
5. Verify rendered output on `/j/<city>/<topic>`

## Commands

Start app dependencies:

```bash
docker compose up -d db server
```

Load markdown content:

```bash
docker compose run --rm ingest
```

Run migration coverage check (legacy HTML vs markdown playbooks):

```bash
make migration-check
```

Run tests:

```bash
go test ./... -race -count=1
```

## Notes

- Editing legacy HTML first is not wasted work. Treat it as drafting, then migrate the final content into markdown.
- Do not delete legacy topic HTML until the Go-rendered routes are verified in deployment.
