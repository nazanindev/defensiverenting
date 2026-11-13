package store

import (
	"context"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// QualityPage is one page on the quality map (ADR-027 D5): where it sits,
// what it is, and every check it fails.
type QualityPage struct {
	ID               int64
	Title            string
	Status           string // draft or published
	PageKind         string
	JurisdictionName string
	JurisdictionSlug string
	JurisdictionKind string
	// TopicSlug is the core topic the page sits under on the map. A rules
	// page sits under the situation topic it answers (ADR-028 D4).
	TopicSlug string
	// Statements is how many statements the page has, so a statement-by-
	// check grid has a row for every one, passing or not.
	Statements int
	// Labels is the opening words of each statement, in page order, so a
	// grid row is named by what the statement says.
	Labels []string
	Checks []QualityCheck
}

// Worst is the severity of the page's worst failing check, 0 when it passes.
func (p QualityPage) Worst() int {
	if len(p.Checks) == 0 {
		return 0
	}
	return p.Checks[0].Severity
}

// AuthorQuality runs the quality checks over every draft and published page
// in an active language and returns them, places in map order (the country
// first, then states, then cities, each by name) and topics by slug.
//
// Drafts are judged alongside live pages on purpose: a draft is where an
// agent can still fix what the map shows (Nazanin, 2026-10-11).
func (pg *PG) AuthorQuality(ctx context.Context) ([]QualityPage, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id, pb.title, pb.status, pb.page_kind, pb.language,
		       j.name, j.slug, j.kind,
		       COALESCE(rt.slug, t.slug)
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics t ON t.id = pb.topic_id
		LEFT JOIN topics rt ON rt.id = t.rules_for
		WHERE pb.status IN ('draft', 'published')
		  AND pb.language = ANY($1)
		ORDER BY (j.kind = 'country') DESC, (j.kind = 'state') DESC, j.name, COALESCE(rt.slug, t.slug), pb.status`,
		ContentLanguages)
	if err != nil {
		return nil, err
	}
	pages := []QualityPage{}
	index := map[int64]int{}
	inputs := map[int64]*QualityInput{}
	for rows.Next() {
		var p QualityPage
		var lang string
		if err := rows.Scan(&p.ID, &p.Title, &p.Status, &p.PageKind, &lang,
			&p.JurisdictionName, &p.JurisdictionSlug, &p.JurisdictionKind, &p.TopicSlug); err != nil {
			rows.Close()
			return nil, err
		}
		index[p.ID] = len(pages)
		pages = append(pages, p)
		inputs[p.ID] = &QualityInput{Language: lang, PageKind: p.PageKind, Warns: map[string]bool{}}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Statements in page order, each with its citations. One pass over the
	// join: a statement with no citation still arrives, with NULL source
	// columns, so the "no citation" tier is honest.
	srows, err := pg.pool.Query(ctx, `
		SELECT ps.playbook_id, ps.position, s.body_md,
		       COALESCE(src.kind, ''), COALESCE(src.url, ''), COALESCE(c.locator, ''),
		       COALESCE(c.quote, ''),
		       COALESCE(c.checked_at IS NOT NULL OR c.manually_verified, false),
		       COALESCE(c.checked_via, '')
		FROM playbook_statements ps
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN statements s ON s.id = ps.statement_id
		LEFT JOIN citations c ON c.statement_id = s.id
		LEFT JOIN sources src ON src.id = c.source_id
		WHERE pb.status IN ('draft', 'published') AND pb.language = ANY($1)
		ORDER BY ps.playbook_id, ps.position, c.id`,
		ContentLanguages)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	lastPage, lastPos := int64(-1), -1
	for srows.Next() {
		var pid int64
		var pos int
		var body string
		var c QualityCitation
		if err := srows.Scan(&pid, &pos, &body, &c.SourceKind, &c.URL, &c.Locator, &c.Quote, &c.Checked, &c.CheckedVia); err != nil {
			return nil, err
		}
		in, ok := inputs[pid]
		if !ok {
			continue
		}
		if pid != lastPage || pos != lastPos {
			in.Statements = append(in.Statements, QualityStatement{Body: body})
			lastPage, lastPos = pid, pos
		}
		if c.SourceKind != "" {
			st := &in.Statements[len(in.Statements)-1]
			st.Citations = append(st.Citations, c)
		}
	}
	if err := srows.Err(); err != nil {
		return nil, err
	}

	// The risk a page says once at the top (ADR-016 A3), so a statement
	// that relies on it is not flagged.
	wrows, err := pg.pool.Query(ctx, `
		SELECT pa.playbook_id, a.warns FROM playbook_advice pa
		JOIN advice a ON a.id = pa.advice_id
		WHERE a.warns IS NOT NULL AND a.retired_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer wrows.Close()
	for wrows.Next() {
		var pid int64
		var w string
		if err := wrows.Scan(&pid, &w); err != nil {
			return nil, err
		}
		if in, ok := inputs[pid]; ok {
			in.Warns[strings.TrimSpace(w)] = true
		}
	}
	if err := wrows.Err(); err != nil {
		return nil, err
	}

	for id, in := range inputs {
		pages[index[id]].Statements = len(in.Statements)
		labels := make([]string, len(in.Statements))
		for k, st := range in.Statements {
			labels[k] = statementLabel(st.Body)
		}
		pages[index[id]].Labels = labels
		pages[index[id]].Checks = pg.quality.checks(id, in)
	}
	pg.quality.keep(inputs)

	// The topic's shape (ADR-036): a page of a topic with a shape answers
	// each must-answer question. Not memoized: it reads the shape, the
	// stored numbers and the no-law records, none of which are in a page's
	// hash, and it is one query.
	must, err := pg.mustAnswerChecks(ctx)
	if err != nil {
		return nil, err
	}
	// What is waiting on a page and is a finding, not a decision: a quote
	// gone from its source, a source that changed, an open note or flag.
	// The monitor shows these so the queue does not have to.
	waiting, err := pg.waitingChecks(ctx)
	if err != nil {
		return nil, err
	}
	for id, w := range waiting {
		must[id] = append(must[id], w...)
	}
	for id, extra := range must {
		i, ok := index[id]
		if !ok {
			continue
		}
		// A new slice: the memo owns the one it returned.
		all := append(append([]QualityCheck{}, pages[i].Checks...), extra...)
		sort.SliceStable(all, func(a, b int) bool { return all[a].Severity > all[b].Severity })
		pages[i].Checks = all
	}
	return pages, nil
}

// waitingChecks returns, per page, a failing check for each stored finding
// on it:
//
//   - quote-missing: the source check did not find a statement's quote at
//     the source, or did not find the quote behind a stored number;
//   - source-changed: the source check saw the text around a quote change,
//     and nobody has re-read the statement;
//   - open-note: a note on a statement, or a flag on the page, that nobody
//     has answered.
//
// A waiting proposal that carries a change of its own (an agent's edit, a
// tag change) is a decision, not a finding, and is left to the queue.
func (pg *PG) waitingChecks(ctx context.Context) (map[int64][]QualityCheck, error) {
	out := map[int64][]QualityCheck{}
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id,
		       (SELECT count(*) FROM playbook_statements p2
		        WHERE p2.playbook_id = pb.id AND p2.position <= ps.position)::int,
		       sp.reason, sp.proposed_by,
		       sp.proposed IS NOT NULL AND sp.proposed::text NOT IN ('null', '{}'),
		       COALESCE(sp.evidence->>'note', '')
		FROM statement_proposals sp
		JOIN statements s ON s.key = sp.statement_key
		JOIN playbook_statements ps ON ps.statement_id = s.id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		WHERE sp.status = 'pending'
		  AND (sp.reason = $2 OR sp.reason = 'agent-pass:flag')
		  AND pb.status IN ('draft', 'published') AND pb.language = ANY($1)
		ORDER BY pb.id, ps.position, sp.id`,
		ContentLanguages, ReasonSourceDrift)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var stmt int
		var reason, by, note string
		var hasFix bool
		if err := rows.Scan(&id, &stmt, &reason, &by, &hasFix, &note); err != nil {
			rows.Close()
			return nil, err
		}
		n := strconv.Itoa(stmt)
		switch {
		case reason == ReasonSourceDrift && hasFix:
			out[id] = append(out[id], QualityCheck{Code: "quote-missing", Severity: SeverityOrange, Stmt: stmt,
				Detail: "statement " + n + ": a quote is no longer found at its source; a replacement quote is waiting in the queue"})
		case reason == ReasonSourceDrift:
			out[id] = append(out[id], QualityCheck{Code: "quote-missing", Severity: SeverityOrange, Stmt: stmt,
				Detail: "statement " + n + ": a quote is no longer found at its source, and nothing close to it was found"})
		case by == ActorSourceCheck:
			out[id] = append(out[id], QualityCheck{Code: "source-changed", Severity: SeverityYellow, Stmt: stmt,
				Detail: "statement " + n + ": the source changed around a quote since it was last checked; read the statement against the section as it reads now"})
		default:
			out[id] = append(out[id], QualityCheck{Code: "open-note", Severity: SeverityYellow, Stmt: stmt,
				Detail: "statement " + n + " has an open note: " + clipNote(note)})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	frows, err := pg.pool.Query(ctx, `
		SELECT f.playbook_id, f.note FROM page_flags f
		JOIN playbooks pb ON pb.id = f.playbook_id
		WHERE f.status = 'open' AND pb.status IN ('draft', 'published') AND pb.language = ANY($1)
		ORDER BY f.playbook_id, f.id`, ContentLanguages)
	if err != nil {
		return nil, err
	}
	for frows.Next() {
		var id int64
		var note string
		if err := frows.Scan(&id, &note); err != nil {
			frows.Close()
			return nil, err
		}
		out[id] = append(out[id], QualityCheck{Code: "open-note", Severity: SeverityOrange,
			Detail: "the page has an open flag, which blocks publishing: " + clipNote(note)})
	}
	frows.Close()
	if err := frows.Err(); err != nil {
		return nil, err
	}

	// A stored number whose quote the source check did not find, on each
	// page that answers that question for that place.
	vrows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT ON (pb.id, rv.id) pb.id,
		       (SELECT count(*) FROM playbook_statements p2
		        WHERE p2.playbook_id = pb.id AND p2.position <= ps.position)::int,
		       c.question
		FROM rule_values rv
		JOIN concepts c ON c.id = rv.concept_id
		JOIN playbooks pb ON pb.jurisdiction_id = rv.jurisdiction_id
		JOIN playbook_statements ps ON ps.playbook_id = pb.id
		JOIN statements s ON s.id = ps.statement_id AND s.concept_id = rv.concept_id
		WHERE rv.drift_at IS NOT NULL
		  AND pb.status IN ('draft', 'published') AND pb.language = ANY($1)
		ORDER BY pb.id, rv.id, ps.position`, ContentLanguages)
	if err != nil {
		return nil, err
	}
	defer vrows.Close()
	for vrows.Next() {
		var id int64
		var stmt int
		var question string
		if err := vrows.Scan(&id, &stmt, &question); err != nil {
			return nil, err
		}
		out[id] = append(out[id], QualityCheck{Code: "quote-missing", Severity: SeverityYellow, Stmt: stmt,
			Detail: "statement " + strconv.Itoa(stmt) + ": the quote behind the stored number for \"" + question + "\" was not found at its source"})
	}
	return out, vrows.Err()
}

// clipNote shortens a note to one readable line for a grid cell.
func clipNote(s string) string {
	const limit = 200
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return s
}

// mustAnswerChecks returns, per page, one failing check for each
// must-answer question of the page's topic that the page leaves open
// (ADR-036 D3, D7):
//
//   - no statement on the page answers it, and the place has no "searched,
//     no law found" record for it; or
//   - the question is answered in days, a statement answers it, and no
//     number is stored for the place.
//
// Only state and city pages of a topic with a shape are judged. A
// nationwide page answers in general words.
func (pg *PG) mustAnswerChecks(ctx context.Context) (map[int64][]QualityCheck, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id, c.question, q.answer,
		       COALESCE(st.rank, 0),
		       EXISTS (SELECT 1 FROM coverage_records r
		               WHERE r.jurisdiction_id = pb.jurisdiction_id AND r.concept_id = q.concept_id),
		       EXISTS (SELECT 1 FROM rule_values rv
		               WHERE rv.jurisdiction_id = pb.jurisdiction_id AND rv.concept_id = q.concept_id)
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id AND j.kind IN ('state', 'city')
		JOIN topic_questions q ON q.topic_id = pb.topic_id AND q.role = 'must'
		JOIN concepts c ON c.id = q.concept_id
		LEFT JOIN LATERAL (
			-- The first statement on the page tagged with the question, as its
			-- 1-based place among the page's statements.
			SELECT (SELECT count(*) FROM playbook_statements p2
			        WHERE p2.playbook_id = pb.id AND p2.position <= ps.position)::int AS rank
			FROM playbook_statements ps
			JOIN statements s ON s.id = ps.statement_id
			WHERE ps.playbook_id = pb.id AND s.concept_id = q.concept_id
			ORDER BY ps.position
			LIMIT 1
		) st ON true
		WHERE pb.status IN ('draft', 'published') AND pb.language = ANY($1)
		ORDER BY pb.id, q.position`,
		ContentLanguages)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]QualityCheck{}
	for rows.Next() {
		var id int64
		var question, answer string
		var stmt int
		var noLaw, hasNumber bool
		if err := rows.Scan(&id, &question, &answer, &stmt, &noLaw, &hasNumber); err != nil {
			return nil, err
		}
		switch {
		case stmt == 0 && !noLaw:
			out[id] = append(out[id], QualityCheck{Code: "must-answer", Severity: SeverityOrange,
				Detail: "the page does not answer: " + question})
		case stmt > 0 && answer == "days" && !hasNumber:
			out[id] = append(out[id], QualityCheck{Code: "must-answer", Severity: SeverityOrange, Stmt: stmt,
				Detail: "statement " + strconv.Itoa(stmt) + " answers \"" + question + "\" and no number of days is stored for this place"})
		}
	}
	return out, rows.Err()
}

// qualityMemo holds the last computed checks per page, keyed by a hash of
// everything the checks read. Running the checks over every page costs
// seconds of regex per request (2026-11-11: about 3s for 892 pages), and
// almost no page changes between two clicks on the map. A page whose hash
// matches reuses its checks; any other page is checked again. Nothing here
// can go stale: a changed statement, citation or page warning changes the
// hash.
type qualityMemo struct {
	mu sync.Mutex
	m  map[int64]qualityMemoEntry
}

type qualityMemoEntry struct {
	key    uint64
	checks []QualityCheck
}

func (q *qualityMemo) checks(id int64, in *QualityInput) []QualityCheck {
	key := in.hash()
	q.mu.Lock()
	e, ok := q.m[id]
	q.mu.Unlock()
	if ok && e.key == key {
		return e.checks
	}
	checks := PageQualityChecks(*in)
	q.mu.Lock()
	if q.m == nil {
		q.m = map[int64]qualityMemoEntry{}
	}
	q.m[id] = qualityMemoEntry{key: key, checks: checks}
	q.mu.Unlock()
	return checks
}

// keep drops pages that are no longer on the map (deleted, archived).
func (q *qualityMemo) keep(inputs map[int64]*QualityInput) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for id := range q.m {
		if _, ok := inputs[id]; !ok {
			delete(q.m, id)
		}
	}
}

// hash folds every field the checks read into one number. Field order and
// a separator between fields keep two different inputs from folding alike.
func (in *QualityInput) hash() uint64 {
	h := fnv.New64a()
	w := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	w(in.Language, in.PageKind)
	warns := make([]string, 0, len(in.Warns))
	for k, v := range in.Warns {
		if v {
			warns = append(warns, k)
		}
	}
	sort.Strings(warns)
	w(warns...)
	w("|")
	for _, st := range in.Statements {
		w(st.Body, strconv.Itoa(len(st.Citations)))
		for _, c := range st.Citations {
			w(c.SourceKind, c.URL, c.Locator, c.Quote, strconv.FormatBool(c.Checked), c.CheckedVia)
		}
	}
	return h.Sum64()
}

// statementLabel is the opening words of a statement, cut at a word, short
// enough to name a row.
func statementLabel(body string) string {
	const limit = 70
	body = strings.Join(strings.Fields(body), " ")
	r := []rune(body)
	if len(r) <= limit {
		return body
	}
	cut := limit
	for cut > 0 && r[cut] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = limit
	}
	return strings.TrimRight(string(r[:cut]), " ,;:") + "…"
}
