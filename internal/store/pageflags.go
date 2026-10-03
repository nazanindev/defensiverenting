package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Page flags (ADR-025, amended 2026-10-03). A page flag is a doubt about the
// page as a whole: a gap, a duplicate pair, an intro that misleads, a page
// too thin to publish. It is filed against the page, not its first
// statement, and an open one holds the page from publishing until it is
// closed. Closing records who closed it and why.

// PageFlag is one flag on one page.
type PageFlag struct {
	ID         int64
	PlaybookID int64
	Kind       string
	Note       string
	Keys       []string
	FiledBy    string
	Status     string
	ClosedBy   string
	ClosedAt   *time.Time
	CloseNote  string
	CreatedAt  time.Time
}

// FiledByPerson reports whether a person filed the flag, in which case only
// a person closes it.
func (f PageFlag) FiledByPerson() bool { return isReviewer(f.FiledBy) }

// PageFlagDrafter is the kind of the drafting agent's own page doubt, filed
// from save_draft_playbook's page_note.
const PageFlagDrafter = "drafter"

// ErrPageFlagNotOpen reports a close of a flag that is closed or missing.
var ErrPageFlagNotOpen = errors.New("page flag is not open")

// FilePageFlag records a flag on a page. Nothing is filed when the same note
// is already on file for that page in any status, so a re-run of a findings
// file never stacks duplicates or reopens a flag someone closed. It returns
// the flag's id and whether it was written.
func (pg *PG) FilePageFlag(ctx context.Context, playbookID int64, kind, note string, keys []string, by string) (int64, bool, error) {
	var id int64
	var filed bool
	err := pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		id, filed, err = insertPageFlag(ctx, tx, playbookID, kind, note, keys, by)
		return err
	})
	return id, filed, err
}

func insertPageFlag(ctx context.Context, tx pgx.Tx, playbookID int64, kind, note string, keys []string, by string) (int64, bool, error) {
	kind, note, by = strings.TrimSpace(kind), strings.TrimSpace(note), strings.TrimSpace(by)
	if note == "" {
		return 0, false, nil
	}
	if kind == "" {
		return 0, false, errors.New("a page flag needs a kind")
	}
	if by == "" {
		return 0, false, errors.New("filed_by is required")
	}
	clean := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.ToLower(strings.TrimSpace(k))
		if !uuidRE.MatchString(k) {
			return 0, false, fmt.Errorf("statement key %q is not a statement key", k)
		}
		clean = append(clean, k)
	}
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM page_flags WHERE playbook_id = $1 AND note = $2`, playbookID, note).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, fmt.Errorf("look for an existing page flag: %w", err)
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO page_flags (playbook_id, kind, note, keys, filed_by)
		VALUES ($1, $2, $3, $4::uuid[], $5) RETURNING id`,
		playbookID, kind, note, clean, by).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("file page flag: %w", err)
	}
	return id, true, nil
}

// ErrPageFlagPersons reports an agent closing a flag a person filed.
var ErrPageFlagPersons = errors.New("a flag a person filed is closed by a person")

// ClosePageFlag closes one open flag. by is required. An agent closing a
// flag must say what it did about it, and may close only a flag an agent
// filed: a person's doubt about a page is answered by a person.
func (pg *PG) ClosePageFlag(ctx context.Context, id int64, by, note string) error {
	by, note = strings.TrimSpace(by), strings.TrimSpace(note)
	if by == "" {
		return errors.New("closed_by is required")
	}
	if !isReviewer(by) && note == "" {
		return errors.New("an agent closing a page flag says what it did about it")
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var filedBy string
		err := tx.QueryRow(ctx, `SELECT filed_by FROM page_flags WHERE id = $1 AND status = 'open' FOR UPDATE`, id).Scan(&filedBy)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPageFlagNotOpen
		}
		if err != nil {
			return err
		}
		if !isReviewer(by) && (PageFlag{FiledBy: filedBy}).FiledByPerson() {
			return ErrPageFlagPersons
		}
		_, err = tx.Exec(ctx, `
			UPDATE page_flags SET status = 'closed', closed_by = $2, closed_at = NOW(), close_note = $3
			WHERE id = $1`, id, by, note)
		return err
	})
}

// PageFlagByID returns one flag in any status.
func (pg *PG) PageFlagByID(ctx context.Context, id int64) (PageFlag, error) {
	flags, err := pg.listPageFlags(ctx, `WHERE id = $1`, id)
	if err != nil {
		return PageFlag{}, err
	}
	if len(flags) == 0 {
		return PageFlag{}, ErrNotFound
	}
	return flags[0], nil
}

// OpenPageFlags returns the open flags on one page, oldest first.
func (pg *PG) OpenPageFlags(ctx context.Context, playbookID int64) ([]PageFlag, error) {
	return pg.listPageFlags(ctx, `WHERE playbook_id = $1 AND status = 'open'`, playbookID)
}

// AllOpenPageFlags returns every open flag, by page then age.
func (pg *PG) AllOpenPageFlags(ctx context.Context) ([]PageFlag, error) {
	return pg.listPageFlags(ctx, `WHERE status = 'open'`)
}

func (pg *PG) listPageFlags(ctx context.Context, where string, args ...any) ([]PageFlag, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT id, playbook_id, kind, note, keys::text[], filed_by, status, closed_by, closed_at, close_note, created_at
		FROM page_flags `+where+` ORDER BY playbook_id, created_at, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PageFlag
	for rows.Next() {
		var f PageFlag
		if err := rows.Scan(&f.ID, &f.PlaybookID, &f.Kind, &f.Note, &f.Keys, &f.FiledBy, &f.Status, &f.ClosedBy, &f.ClosedAt, &f.CloseNote, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
