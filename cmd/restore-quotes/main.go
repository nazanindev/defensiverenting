// restore-quotes puts back the quotes an approval dropped before migration
// 000044. Until then a statement could cite a source once, so a proposal that
// quoted one page twice (two subsections of one statute, two passages of one
// guide) was saved with only the last quote. Every approved proposal still
// holds its full citation list; this reads them and adds each missing quote
// to the statement it was approved for.
//
//	restore-quotes                 print what would be restored
//	restore-quotes -apply          restore on draft pages
//	restore-quotes -apply -published
//	                               also restore on published pages
//
// A piece is restored only when the page still carries a statement with the
// approved body and every citation it has now is one the proposal listed, so a
// later edit is never overwritten. Each restored quote is fetched live; a
// quote found there is stamped checked by the source check, one that cannot
// be confirmed goes in unchecked for the next check run. Adding evidence
// changes a statement's review hash, so a restored statement returns to
// unreviewed and is read again.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

type cite struct {
	URL     string `json:"url"`
	Locator string `json:"locator"`
	Quote   string `json:"quote"`
}

type piece struct {
	BodyMD    string  `json:"body_md"`
	Citations []cite  `json:"citations"`
	Followers []piece `json:"followers"`
}

type current struct {
	id     int64
	cites  map[string]bool // url + "\x00" + quote
	status string
	page   string
	pos    int
}

func main() {
	apply := flag.Bool("apply", false, "write the restored quotes; default prints them")
	published := flag.Bool("published", false, "also restore on published pages")
	flag.Parse()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fatal(err)
	}
	defer c.Close(ctx)

	rows, err := c.Query(ctx, `
		SELECT id, playbook_id, proposed FROM statement_proposals
		WHERE status = 'approved' AND jsonb_typeof(proposed) = 'object'
		ORDER BY decided_at`)
	if err != nil {
		fatal(err)
	}
	type prop struct {
		id, playbook int64
		p            piece
	}
	var props []prop
	for rows.Next() {
		var pr prop
		var raw []byte
		if err := rows.Scan(&pr.id, &pr.playbook, &raw); err != nil {
			fatal(err)
		}
		if err := json.Unmarshal(raw, &pr.p); err != nil {
			fatal(fmt.Errorf("proposal #%d: %w", pr.id, err))
		}
		props = append(props, pr)
	}
	if err := rows.Err(); err != nil {
		fatal(err)
	}

	check := drafting.LiveQuoteCheck()
	var restored, skipped, held, checked int
	for _, pr := range props {
		pieces := append([]piece{pr.p}, pr.p.Followers...)
		for _, pc := range pieces {
			if !hasRepeatedURL(pc.Citations) {
				continue
			}
			cur, err := find(ctx, c, pr.playbook, pc.BodyMD)
			if err != nil {
				fatal(err)
			}
			if cur == nil {
				fmt.Printf("#%d skip: no statement on playbook %d carries the approved body any more: %.60q\n", pr.id, pr.playbook, pc.BodyMD)
				skipped++
				continue
			}
			listed := map[string]bool{}
			for _, ct := range pc.Citations {
				listed[ct.URL+"\x00"+ct.Quote] = true
			}
			changed := false
			for k := range cur.cites {
				if !listed[k] {
					changed = true
				}
			}
			if changed {
				fmt.Printf("#%d skip: %s · statement %d was edited after approval\n", pr.id, cur.page, cur.pos)
				skipped++
				continue
			}
			var missing []cite
			for _, ct := range pc.Citations {
				if strings.TrimSpace(ct.Quote) != "" && !cur.cites[ct.URL+"\x00"+ct.Quote] {
					missing = append(missing, ct)
				}
			}
			if len(missing) == 0 {
				continue
			}
			if cur.status == "published" && !*published {
				fmt.Printf("#%d held (published): %s · statement %d would get %d quotes back\n", pr.id, cur.page, cur.pos, len(missing))
				held++
				continue
			}
			for _, m := range missing {
				v := check(ctx, cur.pos, m.URL, m.Quote)
				state := "unchecked"
				if v.Verified {
					state = "confirmed live"
					checked++
				}
				fmt.Printf("#%d restore: %s · statement %d · %s (%s)\n", pr.id, cur.page, cur.pos, m.Locator, state)
				restored++
				if !*apply {
					continue
				}
				if _, err := c.Exec(ctx, `
					INSERT INTO citations (statement_id, source_id, locator, quote,
					                       checked_at, checked_by, checked_via, checked_extractor, checked_hash, checked_context)
					SELECT $1, src.id, $3, $4,
					       CASE WHEN $5 THEN NOW() END, CASE WHEN $5 THEN $6 ELSE '' END,
					       CASE WHEN $5 THEN $7 ELSE '' END, CASE WHEN $5 THEN $8 ELSE '' END,
					       CASE WHEN $5 THEN $9 ELSE '' END, CASE WHEN $5 THEN $10 ELSE '' END
					FROM sources src WHERE src.url = $2
					ON CONFLICT (statement_id, source_id, md5(quote)) DO NOTHING`,
					cur.id, m.URL, m.Locator, m.Quote, v.Verified, store.ActorSourceCheck,
					v.Receipt.Via, v.Receipt.Extractor, v.Receipt.Hash, v.Receipt.Context); err != nil {
					fatal(fmt.Errorf("#%d: %w", pr.id, err))
				}
			}
		}
	}
	verb := "would be restored; nothing written (add -apply)"
	if *apply {
		verb = "restored"
	}
	fmt.Printf("%d quotes %s, %d confirmed live; %d pieces skipped; %d pieces held on published pages\n", restored, verb, checked, skipped, held)
}

func hasRepeatedURL(cs []cite) bool {
	seen := map[string]bool{}
	for _, c := range cs {
		if c.URL == "" {
			continue // editorial guidance carries no URL
		}
		if seen[c.URL] {
			return true
		}
		seen[c.URL] = true
	}
	return false
}

// find returns the statement on the playbook whose body is the approved one,
// with the citations it carries now.
func find(ctx context.Context, c *pgx.Conn, playbook int64, body string) (*current, error) {
	rows, err := c.Query(ctx, `
		SELECT s.id, pb.status, j.name || ' · ' || t.name, ps.position, COALESCE(src.url, ''), COALESCE(ci.quote, '')
		FROM playbook_statements ps
		JOIN statements s ON s.id = ps.statement_id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics t ON t.id = pb.topic_id
		LEFT JOIN citations ci ON ci.statement_id = s.id
		LEFT JOIN sources src ON src.id = ci.source_id
		WHERE ps.playbook_id = $1 AND s.body_md = $2
		ORDER BY s.id`, playbook, body)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cur *current
	for rows.Next() {
		var id int64
		var status, page, url, quote string
		var pos int
		if err := rows.Scan(&id, &status, &page, &pos, &url, &quote); err != nil {
			return nil, err
		}
		if cur == nil {
			cur = &current{id: id, cites: map[string]bool{}, status: status, page: page, pos: pos}
		}
		if id != cur.id {
			continue // the same body twice on one page: restore the first only
		}
		if url != "" {
			cur.cites[url+"\x00"+quote] = true
		}
	}
	return cur, rows.Err()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "restore-quotes:", err)
	os.Exit(1)
}
