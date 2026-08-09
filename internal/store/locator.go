package store

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
)

// Statute citations must name the provision they rely on.
//
// The citation guarantee (ADR-003) makes every claim traceable to a source. It
// does not check that the source is the kind of thing it claims to be, and that
// gap is not theoretical: a county press release announcing a bill was stored
// with kind 'statute' and became the only support for a live claim. A press
// release has no section number, so requiring one separates a pointer at
// statutory text from a pointer at a document that merely discusses it.
//
// The rule is deliberately narrow. It applies to kind 'statute' only, and it
// asks whether the locator names a provision, not whether it names the right
// one. Regulations are cited the same way and are the obvious next candidate;
// they are left out here so this ships against one class of error at a time.

// sectionWords introduce a provision number in ordinary citation forms.
var sectionWords = map[string]bool{
	"sec": true, "secs": true, "section": true, "sections": true,
	"art": true, "article": true, "ch": true, "chapter": true,
	"tit": true, "title": true, "para": true, "paragraph": true,
	"subsec": true, "subsection": true, "rule": true, "reg": true,
	"regulation": true, "cl": true, "clause": true,
}

// looksLikeSection reports whether a locator points at a numbered provision
// rather than at a document as a whole.
//
// Accepted: "§ 15B", "RCW 59.18.060", "59.18.060", "Section 8-107",
// "Mass. Gen. Laws ch. 186, § 15B".
// Rejected: "", "March 2026", "2026", "County announces new protections".
//
// A bare four-digit number is treated as a year rather than a provision, which
// is what keeps a dated press release from passing on its date alone.
func looksLikeSection(locator string) bool {
	s := strings.TrimSpace(locator)
	if s == "" {
		return false
	}
	// A section sign is unambiguous wherever it appears.
	if strings.ContainsRune(s, '§') {
		return true
	}
	fields := strings.Fields(s)
	for i, f := range fields {
		tok := strings.Trim(f, ".,:;()[]")
		if tok == "" {
			continue
		}
		// A locator opening with a provision number: "59.18.060", "15B".
		if i == 0 && startsWithDigit(tok) && !isBareYear(tok) {
			return true
		}
		// A citation word or an all-caps code introducing one: "ch. 186",
		// "RCW 59.18.060". The code must be all caps so that an ordinary
		// capitalised word followed by a year ("March 2026") does not qualify.
		if sectionWords[strings.ToLower(tok)] || isCodeAbbrev(tok) {
			if i+1 < len(fields) && strings.ContainsFunc(fields[i+1], unicode.IsDigit) {
				return true
			}
		}
	}
	return false
}

func startsWithDigit(s string) bool {
	return s != "" && s[0] >= '0' && s[0] <= '9'
}

func isBareYear(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isCodeAbbrev(s string) bool {
	r := []rune(s)
	if len(r) < 2 || len(r) > 6 {
		return false
	}
	for _, c := range r {
		if !unicode.IsUpper(c) {
			return false
		}
	}
	return true
}

// validateStatuteLocators rejects any citation that claims statutory authority
// without naming a provision. It runs inside the ingest transaction so a
// rejected playbook writes nothing, the same way a citation-less statement
// aborts the write (ADR-003, schema layer).
func validateStatuteLocators(ctx context.Context, tx pgx.Tx, statements []IngestStatementParams) error {
	ids := make([]int64, 0, len(statements))
	seen := map[int64]bool{}
	for _, sp := range statements {
		for _, c := range sp.Sources {
			if !seen[c.SourceID] {
				seen[c.SourceID] = true
				ids = append(ids, c.SourceID)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}

	rows, err := tx.Query(ctx, `SELECT id, kind FROM sources WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("load source kinds: %w", err)
	}
	defer rows.Close()
	kinds := make(map[int64]string, len(ids))
	for rows.Next() {
		var id int64
		var kind string
		if err := rows.Scan(&id, &kind); err != nil {
			return fmt.Errorf("scan source kind: %w", err)
		}
		kinds[id] = kind
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load source kinds: %w", err)
	}

	for i, sp := range statements {
		for _, c := range sp.Sources {
			if kinds[c.SourceID] != "statute" {
				continue
			}
			if !looksLikeSection(c.Locator) {
				return fmt.Errorf(
					"statement %d cites source %d as a statute but its locator %q does not name a provision — "+
						"set the locator to the section relied on (for example %q or %q), or change the source "+
						"kind if it is not statutory text",
					i, c.SourceID, c.Locator, "§ 15B", "RCW 59.18.060")
			}
		}
	}
	return nil
}
