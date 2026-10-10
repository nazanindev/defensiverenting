package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Practical advice is a registry pages reference (ADR-016, amended
// 2026-10-09). An entry is written once, by a person, in a migration, and is
// backed by a government or nonprofit quote. A page references entries by
// slug: a tip sits under the statement it helps with; a page note (the
// disclaimer, the risk warning) is said once at the top of the page. Agents
// reference entries; they never write them.

// Advice is one registry entry as a page shows it.
type Advice struct {
	ID        int64
	Slug      string
	Kind      string // "tip" or "page_note"
	Warns     string // a page note's risk: "owe", "evict", or ""
	BodyMD    string
	SiteVoice bool
	Retired   bool
	// StatementKey is the statement a tip sits under on this page; "" for a
	// page note. Set only when the entry is loaded as a page reference.
	StatementKey string
	Citations    []AdviceCitation
}

// adviceAppliesSQL says whether the source aliased s may back advice on the
// page aliased pb: a national source anywhere, a state source on that
// state's pages and its cities', a city source on that city's page. A source
// from another place never backs advice here (ADR-016 A1, 2026-10-09).
const adviceAppliesSQL = `(s.jurisdiction_id IS NULL
	OR EXISTS (SELECT 1 FROM jurisdictions sj WHERE sj.id = s.jurisdiction_id AND sj.kind = 'country')
	OR s.jurisdiction_id = pb.jurisdiction_id
	OR s.jurisdiction_id = (SELECT parent_id FROM jurisdictions WHERE id = pb.jurisdiction_id))`

// adviceLocalSQL ranks a place's own source ahead of a national one.
const adviceLocalSQL = `(s.jurisdiction_id IS NOT NULL AND NOT EXISTS (
	SELECT 1 FROM jurisdictions sj WHERE sj.id = s.jurisdiction_id AND sj.kind = 'country'))`

// AdviceCitation is the quote that backs an entry.
type AdviceCitation struct {
	SourceID  int64
	URL       string
	Publisher string
	Kind      string
	Quote     string
	CheckedAt *time.Time
	DriftAt   *time.Time
	// Place names the source's place, "" for a national source.
	Place string
}

// Backed reports whether an entry may publish: the site's own voice, or at
// least one quote that the checker has found at its source and not since
// lost.
func (a Advice) Backed() bool {
	if a.SiteVoice {
		return true
	}
	for _, c := range a.Citations {
		if c.CheckedAt != nil && c.DriftAt == nil {
			return true
		}
	}
	return false
}

// AdviceRef attaches one entry to one page.
type AdviceRef struct {
	PlaybookID   int64  `json:"playbook_id"`
	Slug         string `json:"slug"`
	StatementKey string `json:"statement_key,omitempty"`
}

// PageAdvice loads a page's references, page notes first and then tips in
// the order the page lists them.
func (pg *PG) PageAdvice(ctx context.Context, playbookID int64) ([]Advice, error) {
	return pageAdvice(ctx, pg.pool, playbookID)
}

func pageAdvice(ctx context.Context, q rowQuerier, playbookID int64) ([]Advice, error) {
	rows, err := q.Query(ctx, `
		SELECT a.id, a.slug, a.kind, COALESCE(a.warns, ''), a.body_md, a.site_voice,
		       a.retired_at IS NOT NULL, COALESCE(pa.statement_key::text, '')
		FROM playbook_advice pa
		JOIN advice a ON a.id = pa.advice_id
		WHERE pa.playbook_id = $1
		ORDER BY (a.kind = 'page_note') DESC, pa.position, a.id`, playbookID)
	if err != nil {
		return nil, err
	}
	var out []Advice
	for rows.Next() {
		var a Advice
		if err := rows.Scan(&a.ID, &a.Slug, &a.Kind, &a.Warns, &a.BodyMD, &a.SiteVoice, &a.Retired, &a.StatementKey); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		cs, err := pageAdviceCitations(ctx, q, out[i].ID, playbookID)
		if err != nil {
			return nil, err
		}
		out[i].Citations = cs
	}
	return out, nil
}

// pageAdviceCitations is an entry's backing as one page may use it: the
// page's own place first, then national; another place's never.
func pageAdviceCitations(ctx context.Context, q rowQuerier, adviceID, playbookID int64) ([]AdviceCitation, error) {
	rows, err := q.Query(ctx, `
		SELECT s.id, s.url, s.publisher, s.kind, c.quote, c.checked_at, c.drift_at, COALESCE(sjn.name, '')
		FROM advice_citations c
		JOIN sources s ON s.id = c.source_id
		JOIN playbooks pb ON pb.id = $2
		LEFT JOIN jurisdictions sjn ON sjn.id = s.jurisdiction_id AND sjn.kind <> 'country'
		WHERE c.advice_id = $1 AND `+adviceAppliesSQL+`
		ORDER BY `+adviceLocalSQL+` DESC, c.position, c.id`, adviceID, playbookID)
	if err != nil {
		return nil, err
	}
	return scanAdviceCitations(rows)
}

// adviceCitations is every quote behind an entry, wherever it applies.
func adviceCitations(ctx context.Context, q rowQuerier, adviceID int64) ([]AdviceCitation, error) {
	rows, err := q.Query(ctx, `
		SELECT s.id, s.url, s.publisher, s.kind, c.quote, c.checked_at, c.drift_at, COALESCE(sjn.name, '')
		FROM advice_citations c
		JOIN sources s ON s.id = c.source_id
		LEFT JOIN jurisdictions sjn ON sjn.id = s.jurisdiction_id AND sjn.kind <> 'country'
		WHERE c.advice_id = $1
		ORDER BY c.position, c.id`, adviceID)
	if err != nil {
		return nil, err
	}
	return scanAdviceCitations(rows)
}

func scanAdviceCitations(rows pgx.Rows) ([]AdviceCitation, error) {
	defer rows.Close()
	var out []AdviceCitation
	for rows.Next() {
		var c AdviceCitation
		if err := rows.Scan(&c.SourceID, &c.URL, &c.Publisher, &c.Kind, &c.Quote, &c.CheckedAt, &c.DriftAt, &c.Place); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PageWarns returns the risks the page states once at the top ("owe",
// "evict"), so the lint can accept a risky statement without its own
// warning. A retired note warns nothing.
func (pg *PG) PageWarns(ctx context.Context, playbookID int64) (map[string]bool, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT a.warns FROM playbook_advice pa
		JOIN advice a ON a.id = pa.advice_id
		WHERE pa.playbook_id = $1 AND a.warns IS NOT NULL AND a.retired_at IS NULL`, playbookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			return nil, err
		}
		out[w] = true
	}
	return out, rows.Err()
}

// ErrAdviceRef is a reference the registry or the page cannot take.
var ErrAdviceRef = errors.New("advice reference refused")

// SetPageAdvice attaches entries to pages. Every reference is checked before
// any is written: the slug must name a live entry, a tip must name a
// statement on that page, a page note must name none, and only draft pages
// take agent references (live pages stay human). One call, one transaction.
func (pg *PG) SetPageAdvice(ctx context.Context, refs []AdviceRef, by string) error {
	if by == "" {
		return errors.New("created_by is required")
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		for i, r := range refs {
			var adviceID int64
			var kind string
			var retired bool
			err := tx.QueryRow(ctx, `SELECT id, kind, retired_at IS NOT NULL FROM advice WHERE slug = $1`, r.Slug).
				Scan(&adviceID, &kind, &retired)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: entry %d: no advice %q in the registry", ErrAdviceRef, i+1, r.Slug)
			}
			if err != nil {
				return err
			}
			if retired {
				return fmt.Errorf("%w: entry %d: advice %q is retired", ErrAdviceRef, i+1, r.Slug)
			}
			var status string
			if err := tx.QueryRow(ctx, `SELECT status FROM playbooks WHERE id = $1`, r.PlaybookID).Scan(&status); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("%w: entry %d: no page %d", ErrAdviceRef, i+1, r.PlaybookID)
				}
				return err
			}
			if status != "draft" {
				return fmt.Errorf("%w: entry %d: page %d is %s; agents attach advice to drafts only", ErrAdviceRef, i+1, r.PlaybookID, status)
			}
			var key any
			switch kind {
			case "tip":
				if r.StatementKey == "" {
					return fmt.Errorf("%w: entry %d: tip %q needs the statement_key it sits under", ErrAdviceRef, i+1, r.Slug)
				}
				var ok bool
				if err := tx.QueryRow(ctx, `
					SELECT EXISTS (SELECT 1 FROM playbook_statements ps JOIN statements s ON s.id = ps.statement_id
					               WHERE ps.playbook_id = $1 AND s.key::text = $2)`, r.PlaybookID, r.StatementKey).Scan(&ok); err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("%w: entry %d: statement %s is not on page %d", ErrAdviceRef, i+1, r.StatementKey, r.PlaybookID)
				}
				key = r.StatementKey
			case "page_note":
				if r.StatementKey != "" {
					return fmt.Errorf("%w: entry %d: page note %q belongs to the page, not a statement", ErrAdviceRef, i+1, r.Slug)
				}
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO playbook_advice (playbook_id, advice_id, statement_key, position, created_by)
				VALUES ($1, $2, $3, (SELECT COALESCE(max(position), -1) + 1 FROM playbook_advice WHERE playbook_id = $1), $4)
				ON CONFLICT (playbook_id, advice_id) DO UPDATE SET statement_key = EXCLUDED.statement_key`,
				r.PlaybookID, adviceID, key, by); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemovePageAdvice detaches entries from draft pages.
func (pg *PG) RemovePageAdvice(ctx context.Context, refs []AdviceRef) error {
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		for i, r := range refs {
			tag, err := tx.Exec(ctx, `
				DELETE FROM playbook_advice pa USING advice a, playbooks pb
				WHERE pa.advice_id = a.id AND pb.id = pa.playbook_id
				  AND pa.playbook_id = $1 AND a.slug = $2 AND pb.status = 'draft'`, r.PlaybookID, r.Slug)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return fmt.Errorf("%w: entry %d: page %d has no draft reference to %q", ErrAdviceRef, i+1, r.PlaybookID, r.Slug)
			}
		}
		return nil
	})
}

// AdviceCheckRow is one advice quote for the source checker.
type AdviceCheckRow struct {
	ID       int64
	Slug     string
	SourceID int64
	URL      string
	Quote    string
}

// ListAdviceCitationsForCheck returns every quote backing a live entry.
func (pg *PG) ListAdviceCitationsForCheck(ctx context.Context) ([]AdviceCheckRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT c.id, a.slug, s.id, s.url, c.quote
		FROM advice_citations c
		JOIN advice a ON a.id = c.advice_id
		JOIN sources s ON s.id = c.source_id
		WHERE a.retired_at IS NULL
		ORDER BY s.url, c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdviceCheckRow
	for rows.Next() {
		var r AdviceCheckRow
		if err := rows.Scan(&r.ID, &r.Slug, &r.SourceID, &r.URL, &r.Quote); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkAdviceCitation records one check of one advice quote: found clears any
// drift and stamps the time; missing records the drift once, on the entry,
// for every page that references it.
func (pg *PG) MarkAdviceCitation(ctx context.Context, id int64, found bool, note string) error {
	if found {
		_, err := pg.pool.Exec(ctx, `
			UPDATE advice_citations SET checked_at = now(), drift_at = NULL, drift_note = '' WHERE id = $1`, id)
		return err
	}
	_, err := pg.pool.Exec(ctx, `
		UPDATE advice_citations SET drift_at = COALESCE(drift_at, now()), drift_note = $2 WHERE id = $1`, id, note)
	return err
}

// KeyPageWarns returns the risks stated at the top of the page a proposal
// for this statement would land on: playbookID when given, otherwise the
// page carrying the key, a draft before a live page, as proposals resolve.
func (pg *PG) KeyPageWarns(ctx context.Context, key string, playbookID int64) (map[string]bool, error) {
	if playbookID == 0 {
		err := pg.pool.QueryRow(ctx, `
			SELECT pb.id FROM playbook_statements ps
			JOIN statements s ON s.id = ps.statement_id
			JOIN playbooks pb ON pb.id = ps.playbook_id
			WHERE s.key::text = lower(btrim($1)) AND pb.status IN ('draft', 'published')
			ORDER BY (pb.status = 'draft') DESC LIMIT 1`, key).Scan(&playbookID)
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]bool{}, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return pg.PageWarns(ctx, playbookID)
}

// ListAdvice returns every live registry entry with its backing quotes, for
// agents choosing which entry to reference.
func (pg *PG) ListAdvice(ctx context.Context) ([]Advice, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT id, slug, kind, COALESCE(warns, ''), body_md, site_voice, false, ''
		FROM advice WHERE retired_at IS NULL ORDER BY (kind = 'page_note') DESC, slug`)
	if err != nil {
		return nil, err
	}
	var out []Advice
	for rows.Next() {
		var a Advice
		if err := rows.Scan(&a.ID, &a.Slug, &a.Kind, &a.Warns, &a.BodyMD, &a.SiteVoice, &a.Retired, &a.StatementKey); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Citations, err = adviceCitations(ctx, pg.pool, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// SlotPageWarns returns the risks stated at the top of the draft in this
// slot, if one exists, so re-saving a draft lints it against its own page.
func (pg *PG) SlotPageWarns(ctx context.Context, jurisdictionSlug, topicSlug, language string) (map[string]bool, error) {
	var id int64
	err := pg.pool.QueryRow(ctx, `
		SELECT pb.id FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics t ON t.id = pb.topic_id
		WHERE j.slug = $1 AND t.slug = $2 AND pb.language = $3 AND pb.status = 'draft'`,
		jurisdictionSlug, topicSlug, language).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	return pg.PageWarns(ctx, id)
}

// pageWarningsSQL is the warning notes on the page aliased pb, one
// "warns<TAB>body" line each, for a statement shown away from its page
// (ADR-016 A3): a concept page or a rules page brings the warning along.
const pageWarningsSQL = `COALESCE((
	SELECT string_agg(a.warns || E'\t' || a.body_md, E'\n' ORDER BY a.warns)
	FROM playbook_advice pa JOIN advice a ON a.id = pa.advice_id
	WHERE pa.playbook_id = pb.id AND a.warns IS NOT NULL AND a.retired_at IS NULL), '')`

// parsePageWarnings turns pageWarningsSQL's text into warns -> body.
func parsePageWarnings(s string) map[string]string {
	if s == "" {
		return nil
	}
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(line, "\t"); ok {
			out[k] = v
		}
	}
	return out
}
