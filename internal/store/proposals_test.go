package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// A proposal is a per-statement change waiting for a person (ADR-014). These
// tests pin down what filing, approving, rejecting, and snoozing do to the
// queue and to the page.

func firstKey(t *testing.T, pg *store.PG, id int64) string {
	t.Helper()
	return statementKeys(t, pg, id)[0]
}

func sourceOf(t *testing.T, pg *store.PG, id int64) store.CitationWithSource {
	t.Helper()
	pw, err := pg.AuthorGetPlaybook(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return pw.Statements[0].Citations[0]
}

func file(t *testing.T, pg *store.PG, key string, pageID int64, body string) int64 {
	t.Helper()
	var proposed *store.ProposedStatement
	if body != "" {
		proposed = &store.ProposedStatement{BodyMD: body, Citations: []store.ProposedCitation{{URL: "https://example.gov/x", Quote: "verbatim", Checked: true}}}
	}
	id, err := pg.FileProposal(context.Background(), store.FileProposalParams{
		StatementKey: key, PlaybookID: pageID, Reason: "agent-pass:test", Proposed: proposed, ProposedBy: "test agent",
	})
	if err != nil {
		t.Fatalf("file: %v", err)
	}
	return id
}

// replacement builds the approval's statement citing the page's existing
// source, with the quote confirmed or not as the test needs.
func replacement(src store.CitationWithSource, body string, checked bool) store.IngestStatementParams {
	return store.IngestStatementParams{BodyMD: body, Sources: []store.IngestCitationParams{{
		SourceID: src.SourceID, Locator: "§ 1", Quote: "a brand new quote", CheckedNow: checked, CheckedBy: "test",
	}}}
}

func TestProposal_newerFilingSupersedesOlder(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	page := seedPlaybook(t, pg, jID, tID, "draft", "Props")
	key := firstKey(t, pg, page)

	first := file(t, pg, key, page, "First wording.")
	second := file(t, pg, key, page, "Second wording.")

	pending, err := pg.ListProposals(ctx, "pending")
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, p := range pending {
		if p.StatementKey == key {
			ids = append(ids, p.ID)
		}
	}
	if len(ids) != 1 || ids[0] != second {
		t.Errorf("pending for key = %v, want only the newer %d", ids, second)
	}
	old, err := pg.GetProposal(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != "superseded" {
		t.Errorf("older proposal status %q, want superseded", old.Status)
	}
	if !old.OnPage() || old.CurrentBody != "A claim. Props" || old.Position != 1 {
		t.Errorf("row shows current body %q at %d, want the page's statement at 1", old.CurrentBody, old.Position)
	}
}

func TestProposal_rejectsBadReasonAndKey(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	page := seedPlaybook(t, pg, jID, tID, "draft", "Props")
	key := firstKey(t, pg, page)
	if _, err := pg.FileProposal(ctx, store.FileProposalParams{StatementKey: key, PlaybookID: page, Reason: "because", ProposedBy: "x"}); err == nil {
		t.Error("reason outside the vocabulary was accepted")
	}
	if _, err := pg.FileProposal(ctx, store.FileProposalParams{StatementKey: "nope", PlaybookID: page, Reason: "source-drift", ProposedBy: "x"}); err == nil {
		t.Error("malformed key was accepted")
	}
}

func TestProposal_approveReplacesStatementAndKeepsKey(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	page := seedPlaybook(t, pg, jID, tID, "draft", "Props")
	key := firstKey(t, pg, page)
	src := sourceOf(t, pg, page)
	id := file(t, pg, key, page, "Approved wording.")

	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: id, By: "Nazanin", Statement: replacement(src, "Approved wording.", true)}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	pw, err := pg.AuthorGetPlaybook(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	if len(pw.Statements) != 1 || pw.Statements[0].BodyMD != "Approved wording." || pw.Statements[0].Key != key {
		t.Errorf("page after approval = %+v, want one statement %q under key %s", pw.Statements, "Approved wording.", key)
	}
	if pw.UpdatedBy != "Nazanin" {
		t.Errorf("updated_by = %q, want the reviewer, since approval is their save", pw.UpdatedBy)
	}
	p, err := pg.GetProposal(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != "approved" || p.DecidedBy != "Nazanin" || p.DecidedAt == nil {
		t.Errorf("proposal after approval: status %q by %q at %v", p.Status, p.DecidedBy, p.DecidedAt)
	}
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: id, By: "Nazanin", Statement: replacement(src, "Again.", true)}); !errors.Is(err, store.ErrProposalNotPending) {
		t.Errorf("second approval err = %v, want ErrProposalNotPending", err)
	}
}

func TestProposal_approvalOnLivePageRunsTheGate(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	live := seedPlaybook(t, pg, jID, tID, "published", "Props")
	key := firstKey(t, pg, live)
	src := sourceOf(t, pg, live)
	id := file(t, pg, key, live, "Unverified wording.")

	// The replacement's quote was never confirmed. On a live page that is
	// exactly what the publish gate refuses (ADR-013 D3), and the refusal
	// must leave both the page and the proposal as they were.
	err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: id, By: "Nazanin", Statement: replacement(src, "Unverified wording.", false)})
	var npe *store.NotPublishableError
	if !errors.As(err, &npe) {
		t.Fatalf("approve err = %v, want NotPublishableError", err)
	}
	pw, err := pg.AuthorGetPlaybook(ctx, live)
	if err != nil {
		t.Fatal(err)
	}
	if pw.Statements[0].BodyMD != "A claim. Props" {
		t.Errorf("refused approval still changed the live page to %q", pw.Statements[0].BodyMD)
	}
	p, _ := pg.GetProposal(ctx, id)
	if p.Status != "pending" {
		t.Errorf("proposal status after refusal %q, want pending", p.Status)
	}

	// With the quote confirmed the same approval goes through, and it is a
	// publish: the live page now says the new thing.
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: id, By: "Nazanin", Statement: replacement(src, "Verified wording.", true)}); err != nil {
		t.Fatalf("approve with checked quote: %v", err)
	}
	got, _ := pg.GetPlaybook(ctx, "rev-city-"+t.Name(), "rev-topic-"+t.Name(), "en")
	if got.Statements[0].BodyMD != "Verified wording." {
		t.Errorf("live page reads %q, want the approved wording", got.Statements[0].BodyMD)
	}
}

func TestProposal_appliesToDraftRevisionWhenOneExists(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	live := seedPlaybook(t, pg, jID, tID, "published", "Props")
	key := firstKey(t, pg, live)
	src := sourceOf(t, pg, live)
	// Filed against the live page, before the revision existed.
	id := file(t, pg, key, live, "Revised wording.")
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Props")

	row, err := pg.GetProposal(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if row.TargetPlaybookID != draft || row.TargetStatus != "draft" {
		t.Errorf("target = %d (%s), want the draft revision %d", row.TargetPlaybookID, row.TargetStatus, draft)
	}
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: id, By: "Nazanin", Statement: replacement(src, "Revised wording.", true)}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	d, _ := pg.AuthorGetPlaybook(ctx, draft)
	l, _ := pg.AuthorGetPlaybook(ctx, live)
	if d.Statements[0].BodyMD != "Revised wording." {
		t.Errorf("draft revision reads %q, want the approved wording", d.Statements[0].BodyMD)
	}
	if l.Statements[0].BodyMD != "A claim. Props" {
		t.Errorf("live page was edited to %q; the revision is where the slot's next version is assembled", l.Statements[0].BodyMD)
	}
}

func TestProposal_targetGoneWhenStatementLeftThePage(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	page := seedPlaybook(t, pg, jID, tID, "draft", "Props")
	key := firstKey(t, pg, page)
	src := sourceOf(t, pg, page)
	id := file(t, pg, key, page, "Wording for a statement about to vanish.")

	// The author rewrites the page without carrying the key: a new claim.
	resave(t, pg, page, jID, tID, store.IngestStatementParams{BodyMD: "Something else entirely."})

	row, _ := pg.GetProposal(ctx, id)
	if row.OnPage() {
		t.Errorf("row still claims position %d for a key no longer on the page", row.Position)
	}
	err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: id, By: "Nazanin", Statement: replacement(src, "x", true)})
	if !errors.Is(err, store.ErrProposalTargetGone) {
		t.Errorf("approve err = %v, want ErrProposalTargetGone", err)
	}
}

func TestProposal_rejectAndSnooze(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	page := seedPlaybook(t, pg, jID, tID, "draft", "Props")
	key := firstKey(t, pg, page)

	// A work item: no replacement, just a finding.
	id := file(t, pg, key, page, "")
	if err := pg.DecideProposal(ctx, id, "rejected", "Nazanin", "source is fine, guidance is current", nil); err != nil {
		t.Fatal(err)
	}
	p, _ := pg.GetProposal(ctx, id)
	if p.Status != "rejected" || p.DecisionNote == "" || p.Proposed != nil {
		t.Errorf("rejected work item = status %q note %q proposed %v", p.Status, p.DecisionNote, p.Proposed)
	}
	if err := pg.DecideProposal(ctx, id, "rejected", "Nazanin", "again", nil); !errors.Is(err, store.ErrProposalNotPending) {
		t.Errorf("deciding a decided proposal err = %v, want ErrProposalNotPending", err)
	}

	later := time.Now().Add(48 * time.Hour)
	id2 := file(t, pg, key, page, "Snoozed wording.")
	if err := pg.DecideProposal(ctx, id2, "snoozed", "Nazanin", "", &later); err != nil {
		t.Fatal(err)
	}
	if inList(t, pg, "pending", id2) {
		t.Error("a proposal snoozed until the day after tomorrow is listed as pending")
	}
	if !inList(t, pg, "snoozed", id2) {
		t.Error("snoozed proposal missing from the snoozed listing")
	}
	past := time.Now().Add(-time.Hour)
	if _, err := pg.Pool().Exec(ctx, `UPDATE statement_proposals SET snoozed_until = $2 WHERE id = $1`, id2, past); err != nil {
		t.Fatal(err)
	}
	if !inList(t, pg, "pending", id2) {
		t.Error("a snoozed proposal whose date has come is not back in the pending list")
	}
	n, err := pg.CountPendingProposals(ctx)
	if err != nil || n < 1 {
		t.Errorf("pending count = %d (%v), want at least the returned snooze", n, err)
	}
}

func inList(t *testing.T, pg *store.PG, status string, id int64) bool {
	t.Helper()
	rows, err := pg.ListProposals(context.Background(), status)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// AuthorGetPlaybook used to inner-join citations, so a draft's uncited
// statement (legal since ADR-013) vanished from the edit form on reload and
// would have vanished from the page on a proposal approval. Both read paths
// must carry it.
func TestProposal_approvalKeepsUncitedDraftStatements(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	page := seedPlaybook(t, pg, jID, tID, "draft", "Props")
	key := firstKey(t, pg, page)
	src := sourceOf(t, pg, page)
	if _, err := pg.Pool().Exec(ctx, `
		WITH st AS (INSERT INTO statements (jurisdiction_id, language, body_md) VALUES ($1, 'en', 'Uncited, still being written.') RETURNING id)
		INSERT INTO playbook_statements (playbook_id, statement_id, position) SELECT $2, id, 1 FROM st`, jID, page); err != nil {
		t.Fatal(err)
	}
	pw, err := pg.AuthorGetPlaybook(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	if len(pw.Statements) != 2 || len(pw.Statements[1].Citations) != 0 {
		t.Fatalf("AuthorGetPlaybook returned %d statements, want 2 with the second uncited", len(pw.Statements))
	}
	id := file(t, pg, key, page, "Approved wording.")
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: id, By: "Nazanin", Statement: replacement(src, "Approved wording.", true)}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	pw, _ = pg.AuthorGetPlaybook(ctx, page)
	if len(pw.Statements) != 2 || pw.Statements[1].BodyMD != "Uncited, still being written." {
		t.Errorf("after approval the page has %d statements; the uncited one was dropped", len(pw.Statements))
	}
}

// The checker's two lookups (ADR-014 D4): the statement as a proposal would
// carry it, and whether a drift for this statement has already been seen.
func TestProposal_checkerLookups(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	page := seedPlaybook(t, pg, jID, tID, "published", "Props")
	key := firstKey(t, pg, page)

	st, err := pg.StatementByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if st.BodyMD != "A claim. Props" || len(st.Citations) != 1 || st.Citations[0].Quote != "verbatim" || !st.Citations[0].Checked {
		t.Errorf("StatementByKey = %+v, want the seeded statement with its checked citation", st)
	}
	if _, err := pg.StatementByKey(ctx, "44444444-4444-4444-4444-444444444444"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown key err = %v, want ErrNotFound", err)
	}

	if seen, _ := pg.DriftAlreadyFiled(ctx, key, "verbatim"); seen {
		t.Error("nothing filed yet, but DriftAlreadyFiled says seen")
	}
	id, err := pg.FileProposal(ctx, store.FileProposalParams{StatementKey: key, Reason: "source-drift", ProposedBy: store.ActorSourceCheck, Evidence: []byte(`{"old_quote":"verbatim"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if seen, _ := pg.DriftAlreadyFiled(ctx, key, "verbatim"); !seen {
		t.Error("pending drift not reported as already filed")
	}
	if err := pg.DecideProposal(ctx, id, "rejected", "Nazanin", "source is fine", nil); err != nil {
		t.Fatal(err)
	}
	if seen, _ := pg.DriftAlreadyFiled(ctx, key, "verbatim"); !seen {
		t.Error("a drift rejected for this same quote must not be refiled")
	}
	if seen, _ := pg.DriftAlreadyFiled(ctx, key, "a different quote that vanished later"); seen {
		t.Error("a rejection for one quote must not silence a later, different drift")
	}
}
