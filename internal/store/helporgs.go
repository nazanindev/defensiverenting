package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Help orgs (ADR-029). An org is a website host; every `nonprofit` source
// belongs to the org of its host. A contact-first org shows on the live site
// only after it has said yes. The database functions statement_held_by and
// statement_hidden (migration 000060) are the one place that rule lives;
// everything here reads or feeds them.

// HelpOrg types (ADR-029 D1).
const (
	OrgPublic       = "public"
	OrgContactFirst = "contact_first"
)

// HelpOrg statuses, following from the latest contact attempt (D2).
const (
	OrgNotContacted = "not_contacted"
	OrgContacted    = "contacted"
	OrgMaybe        = "maybe"
	OrgOK           = "ok"
	OrgLeaveOff     = "leave_off"
)

// Contact outcomes and the status each one leaves the org in. Only a yes
// lists an org; no reply, however long, is not a yes.
var outcomeStatus = map[string]string{
	"no_answer":    OrgContacted,
	"left_message": OrgContacted,
	"maybe":        OrgMaybe,
	"yes":          OrgOK,
	"no":           OrgLeaveOff,
}

// HelpOrg is one org with what the Orgs page shows about it.
type HelpOrg struct {
	Host      string
	Name      string
	Type      string
	Status    string
	FollowUp  *time.Time
	UpdatedBy string
	UpdatedAt time.Time
	// Places is every place with a page that cites the org, live or draft.
	Places []string
	// LivePages counts published pages that cite the org.
	LivePages int
	Contacts  []HelpOrgContact
}

// Shown reports whether the org's statements show on the live site.
func (o HelpOrg) Shown() bool { return o.Type == OrgPublic || o.Status == OrgOK }

// Due reports whether the follow-up date the editor set has come.
func (o HelpOrg) Due(now time.Time) bool {
	return o.FollowUp != nil && !o.FollowUp.After(now)
}

// HelpOrgContact is one email or call.
type HelpOrgContact struct {
	ID        int64
	Method    string
	Reached   string
	Outcome   string
	Note      string
	FollowUp  *time.Time
	LoggedBy  string
	CreatedAt time.Time
}

// HelpHiding reports whether the live site hides held statements now (D5).
func (pg *PG) HelpHiding(ctx context.Context) (bool, error) {
	var on bool
	err := pg.pool.QueryRow(ctx, `SELECT hiding FROM help_hiding`).Scan(&on)
	return on, err
}

// SetHelpHiding turns hiding on or off. ADR-029 D5: on only after every Local
// Help page carries public help, so turning it on is refused while any live
// Local Help page would be left short of D8.
func (pg *PG) SetHelpHiding(ctx context.Context, on bool) error {
	if on {
		short, err := pg.LocalHelpShort(ctx)
		if err != nil {
			return err
		}
		if len(short) > 0 {
			return fmt.Errorf("these live Local Help pages have fewer than %d public statements (ADR-029 D8): %s",
				LocalHelpPublicMin, strings.Join(short, ", "))
		}
	}
	_, err := pg.pool.Exec(ctx, `UPDATE help_hiding SET hiding = $1`, on)
	return err
}

// StatementsHeldBy maps each given statement that a not-yet-OK contact-first
// org holds back to that org's host, whether or not hiding is on. A statement
// nothing holds is absent from the map.
func (pg *PG) StatementsHeldBy(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := pg.pool.Query(ctx, `
		SELECT id, statement_held_by(id) FROM unnest($1::bigint[]) AS id
		WHERE statement_held_by(id) IS NOT NULL`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var host string
		if err := rows.Scan(&id, &host); err != nil {
			return nil, err
		}
		out[id] = host
	}
	return out, rows.Err()
}

// HiddenStatements is StatementsHeldBy when hiding is on, and empty when it
// is off: the set the public site leaves out right now.
func (pg *PG) HiddenStatements(ctx context.Context, ids []int64) (map[int64]string, error) {
	on, err := pg.HelpHiding(ctx)
	if err != nil {
		return nil, err
	}
	if !on {
		return map[int64]string{}, nil
	}
	return pg.StatementsHeldBy(ctx, ids)
}

// SyncHelpOrgs adds a row, contact first and not contacted, for every host of
// a `nonprofit` source that has none, named after its most cited publisher.
// The rule already treats a missing row that way; the row makes the org
// visible on the Orgs page so someone can act on it.
func (pg *PG) SyncHelpOrgs(ctx context.Context) error {
	_, err := pg.pool.Exec(ctx, `
		INSERT INTO help_orgs (host, name)
		SELECT DISTINCT ON (h) h, publisher
		FROM (SELECT source_host(src.url) AS h, src.publisher, count(c.statement_id) AS n
		        FROM sources src LEFT JOIN citations c ON c.source_id = src.id
		       WHERE src.kind = 'nonprofit' AND source_host(src.url) IS NOT NULL
		       GROUP BY 1, 2) x
		ORDER BY h, n DESC, length(publisher)
		ON CONFLICT (host) DO NOTHING`)
	return err
}

// ListHelpOrgs returns every org with its places, live page count and
// contact log, in the order the Orgs page shows them (D4): contact-first orgs
// before public ones; among them, a due follow-up first, then orgs not yet
// listed, by how many live pages cite them.
func (pg *PG) ListHelpOrgs(ctx context.Context) ([]HelpOrg, error) {
	if err := pg.SyncHelpOrgs(ctx); err != nil {
		return nil, err
	}
	rows, err := pg.pool.Query(ctx, `
		SELECT o.host, o.name, o.type, o.status, o.follow_up, o.updated_by, o.updated_at,
		       coalesce((SELECT array_agg(DISTINCT j.name ORDER BY j.name)
		                   FROM sources src
		                   JOIN citations c ON c.source_id = src.id
		                   JOIN playbook_statements ps ON ps.statement_id = c.statement_id
		                   JOIN playbooks pb ON pb.id = ps.playbook_id
		                   JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		                  WHERE source_host(src.url) = o.host AND src.kind = 'nonprofit'
		                    AND pb.status IN ('published', 'draft')), '{}'),
		       (SELECT count(DISTINCT pb.id)
		          FROM sources src
		          JOIN citations c ON c.source_id = src.id
		          JOIN playbook_statements ps ON ps.statement_id = c.statement_id
		          JOIN playbooks pb ON pb.id = ps.playbook_id
		         WHERE source_host(src.url) = o.host AND src.kind = 'nonprofit'
		           AND pb.status = 'published')
		FROM help_orgs o`)
	if err != nil {
		return nil, err
	}
	var out []HelpOrg
	idx := map[string]int{}
	for rows.Next() {
		var o HelpOrg
		if err := rows.Scan(&o.Host, &o.Name, &o.Type, &o.Status, &o.FollowUp, &o.UpdatedBy, &o.UpdatedAt,
			&o.Places, &o.LivePages); err != nil {
			rows.Close()
			return nil, err
		}
		idx[o.Host] = len(out)
		out = append(out, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	crows, err := pg.pool.Query(ctx, `
		SELECT host, id, method, reached, outcome, note, follow_up, logged_by, created_at
		FROM help_org_contacts ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	for crows.Next() {
		var host string
		var c HelpOrgContact
		if err := crows.Scan(&host, &c.ID, &c.Method, &c.Reached, &c.Outcome, &c.Note, &c.FollowUp, &c.LoggedBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		if i, ok := idx[host]; ok {
			out[i].Contacts = append(out[i].Contacts, c)
		}
	}
	if err := crows.Err(); err != nil {
		return nil, err
	}
	SortHelpOrgs(out, time.Now())
	return out, nil
}

// SortHelpOrgs puts the orgs in Orgs-page order (see ListHelpOrgs).
func SortHelpOrgs(orgs []HelpOrg, now time.Time) {
	rank := func(o HelpOrg) int {
		switch {
		case o.Type == OrgPublic:
			return 4
		case o.Due(now):
			return 0
		case o.Status == OrgNotContacted || o.Status == OrgContacted || o.Status == OrgMaybe:
			return 1
		case o.Status == OrgOK:
			return 2
		default: // leave off
			return 3
		}
	}
	sort.SliceStable(orgs, func(i, j int) bool {
		a, b := orgs[i], orgs[j]
		ra, rb := rank(a), rank(b)
		if ra != rb {
			return ra < rb
		}
		if a.LivePages != b.LivePages {
			return a.LivePages > b.LivePages
		}
		return a.Name < b.Name
	})
}

// ErrNoHelpOrg reports a change to an org that has no row.
var ErrNoHelpOrg = errors.New("no such org")

// SetHelpOrgType switches an org between public and contact first.
func (pg *PG) SetHelpOrgType(ctx context.Context, host, typ, by string) error {
	if typ != OrgPublic && typ != OrgContactFirst {
		return fmt.Errorf("unknown org type %q", typ)
	}
	if strings.TrimSpace(by) == "" {
		return errors.New("who made the change is required")
	}
	tag, err := pg.pool.Exec(ctx, `
		UPDATE help_orgs SET type = $2, updated_by = $3, updated_at = NOW() WHERE host = $1`, host, typ, by)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoHelpOrg
	}
	return nil
}

// LogHelpOrgContact records one email or call and sets the org's status from
// its outcome, in one transaction, so the status always follows the latest
// attempt. The follow-up date, when given, is the editor's own pick.
func (pg *PG) LogHelpOrgContact(ctx context.Context, host string, c HelpOrgContact) error {
	status, ok := outcomeStatus[c.Outcome]
	if !ok {
		return fmt.Errorf("unknown outcome %q", c.Outcome)
	}
	if c.Method != "email" && c.Method != "call" {
		return fmt.Errorf("unknown method %q", c.Method)
	}
	if strings.TrimSpace(c.LoggedBy) == "" {
		return errors.New("who logged the attempt is required")
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE help_orgs SET status = $2, follow_up = $3, updated_by = $4, updated_at = NOW()
			WHERE host = $1`, host, status, c.FollowUp, c.LoggedBy)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNoHelpOrg
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO help_org_contacts (host, method, reached, outcome, note, follow_up, logged_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			host, c.Method, strings.TrimSpace(c.Reached), c.Outcome, strings.TrimSpace(c.Note), c.FollowUp, c.LoggedBy)
		return err
	})
}

// LocalHelpTopic is the topic of Local Help pages, the one D8 applies to.
const LocalHelpTopic = "resource-directory"

// LocalHelpPublicMin is how many public statements a Local Help page needs
// (ADR-029 D8).
const LocalHelpPublicMin = 3

// localHelpPublicCountSQL counts a page's statements that cite only public
// sources: no `nonprofit` source unless its org is typed public. A statement
// citing a contact-first org does not count even once that org is OK to list,
// because the org can say no later. $1 is the playbook id.
const localHelpPublicCountSQL = `
	SELECT count(*) FROM playbook_statements ps
	WHERE ps.playbook_id = $1
	  AND EXISTS (SELECT 1 FROM citations c WHERE c.statement_id = ps.statement_id)
	  AND NOT EXISTS (
		SELECT 1 FROM citations c
		JOIN sources src ON src.id = c.source_id
		LEFT JOIN help_orgs o ON o.host = source_host(src.url)
		WHERE c.statement_id = ps.statement_id
		  AND src.kind = 'nonprofit'
		  AND coalesce(o.type, 'contact_first') <> 'public')`

// LocalHelpPublicCount returns how many of a page's statements count as
// public help under D8.
func (pg *PG) LocalHelpPublicCount(ctx context.Context, playbookID int64) (int, error) {
	var n int
	err := pg.pool.QueryRow(ctx, localHelpPublicCountSQL, playbookID).Scan(&n)
	return n, err
}

// LocalHelpShort names every live Local Help page with fewer than
// LocalHelpPublicMin public statements, as "Place" strings.
func (pg *PG) LocalHelpShort(ctx context.Context) ([]string, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id, j.name FROM playbooks pb
		JOIN topics t ON t.id = pb.topic_id
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		WHERE t.slug = $1 AND pb.status = 'published'
		ORDER BY j.name`, LocalHelpTopic)
	if err != nil {
		return nil, err
	}
	type page struct {
		id   int64
		name string
	}
	var pages []page
	for rows.Next() {
		var p page
		if err := rows.Scan(&p.id, &p.name); err != nil {
			rows.Close()
			return nil, err
		}
		pages = append(pages, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var short []string
	for _, p := range pages {
		n, err := pg.LocalHelpPublicCount(ctx, p.id)
		if err != nil {
			return nil, err
		}
		if n < LocalHelpPublicMin {
			short = append(short, p.name)
		}
	}
	return short, nil
}
