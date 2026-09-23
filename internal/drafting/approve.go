package drafting

import (
	"context"
	"fmt"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/discover"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// QuoteVerdict is what a quote check answers for one citation of a
// replacement being approved: Verified when the live source was read and
// the quote found in it, with the receipt; Overridable when the source
// could not be read at all, so a proposer's own confirmation may stand in.
// Neither set means the source was read and the quote is not there.
type QuoteVerdict struct {
	Verified    bool
	Overridable bool
	Receipt     store.CheckReceipt
}

// QuoteCheck confirms one quote against its source. stmtNo is the
// statement's position on the page, for messages.
type QuoteCheck func(ctx context.Context, stmtNo int, url, quote string) QuoteVerdict

// ApprovalStore is the slice of the store an approval needs.
type ApprovalStore interface {
	GetEditorialSource(ctx context.Context) (store.Source, error)
	AuthorGetPlaybook(ctx context.Context, id int64) (store.PlaybookWithStatements, error)
	UpsertSource(ctx context.Context, p store.UpsertSourceParams) (store.Source, error)
	ApproveProposal(ctx context.Context, p store.ApproveProposalParams) error
}

// ApplyProposal resolves a replacement's sources, checks its quotes, and
// approves it under an actor's name: the one path an approval takes,
// whether a person clicks Apply on the queue or the statement card, or the
// review agent decides by rule (ADR-021). body overrides the proposed text
// when a reviewer edited it first; note is the decision's record.
//
// Every quote is fetched live. Only when the source cannot be read from
// here does a quote the proposer confirmed in the live page (Checked) go
// through on the proposer's word; a readable page that lacks the quote
// leaves it unverified, and the publish gate says so.
func ApplyProposal(ctx context.Context, pg ApprovalStore, check QuoteCheck, p store.ProposalRow, body, by, note string) error {
	if p.Proposed == nil {
		return fmt.Errorf("proposal #%d carries no replacement", p.ID)
	}
	if body == "" {
		body = p.Proposed.BodyMD
	}
	editorial, err := pg.GetEditorialSource(ctx)
	if err != nil {
		return err
	}
	pw, err := pg.AuthorGetPlaybook(ctx, p.TargetPlaybookID)
	if err != nil {
		return err
	}
	resolve := func(ps store.ProposedStatement, body string) (store.IngestStatementParams, error) {
		stmt := store.IngestStatementParams{BodyMD: body, ConceptSlug: ps.Concept, TopicRefSlug: ps.TopicRef}
		for i, c := range ps.Citations {
			if c.Editorial || c.Kind == "editorial" || strings.TrimSpace(c.URL) == "/editorial" {
				stmt.Sources = append(stmt.Sources, store.IngestCitationParams{SourceID: editorial.ID})
				continue
			}
			u := strings.TrimSpace(c.URL)
			if u == "" {
				return stmt, fmt.Errorf("citation %d has no URL; sources are stored by URL", i+1)
			}
			if discover.ReferenceOnly(u) {
				return stmt, fmt.Errorf("citation %d (%s) is reference-only and can never become a source; reject this proposal or edit the page by hand", i+1, u)
			}
			src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
				URL: u, Publisher: strings.TrimSpace(c.Publisher), Kind: SourceKindOrDefault(c.Kind), JurisdictionID: &pw.JurisdictionID,
			})
			if err != nil {
				return stmt, fmt.Errorf("citation %d: %w", i+1, err)
			}
			cite := store.IngestCitationParams{SourceID: src.ID, Locator: c.Locator, Quote: c.Quote}
			if strings.TrimSpace(c.Quote) != "" {
				v := check(ctx, p.Position, u, c.Quote)
				switch {
				case v.Verified:
					cite.CheckedNow, cite.CheckedBy, cite.Checked = true, by, v.Receipt
				case c.Checked && v.Overridable:
					// This server could not read the page; the proposer did.
					cite.CheckedNow, cite.CheckedBy = true, p.ProposedBy
					cite.Checked = store.CheckReceipt{Via: c.CheckedVia}
				}
			}
			stmt.Sources = append(stmt.Sources, cite)
		}
		return stmt, nil
	}
	stmt, err := resolve(*p.Proposed, body)
	if err != nil {
		return err
	}
	var followers []store.IngestStatementParams
	for i, f := range p.Proposed.Followers {
		fs, err := resolve(f, f.BodyMD)
		if err != nil {
			return fmt.Errorf("follower %d: %w", i+1, err)
		}
		followers = append(followers, fs)
	}
	return pg.ApproveProposal(ctx, store.ApproveProposalParams{
		ID: p.ID, By: by, Statement: stmt, Followers: followers, Note: note,
		Action: p.Proposed.Action, MergeKey: p.Proposed.MergeKey, Order: p.Proposed.Order,
	})
}

// SourceKindOrDefault maps a proposed citation's kind onto the source
// kinds the schema accepts; anything else is government guidance.
func SourceKindOrDefault(k string) string {
	switch k {
	case "statute", "regulation", "gov_guidance", "nonprofit", "court_ruling":
		return k
	}
	return "gov_guidance"
}

// LiveQuoteCheck is a QuoteCheck that fetches each source once per run
// through the same tiers the authoring form uses (direct, then a headless
// render; never an archive snapshot) and matches the quote verbatim.
func LiveQuoteCheck() QuoteCheck {
	type got struct {
		rc  Receipt
		err error
	}
	cache := map[string]got{}
	return func(_ context.Context, _ int, url, quote string) QuoteVerdict {
		g, ok := cache[url]
		if !ok {
			g.rc, g.err = FetchExtract(url)
			cache[url] = g
		}
		if g.err != nil {
			return QuoteVerdict{Overridable: true}
		}
		if QuoteAppearsIn(g.rc.Text, quote) {
			return QuoteVerdict{Verified: true, Receipt: store.CheckReceipt{
				Via: g.rc.Tier, Extractor: g.rc.Extractor, Hash: g.rc.Hash, Context: Context(g.rc.Text, quote),
			}}
		}
		return QuoteVerdict{Overridable: !g.rc.Readable()}
	}
}
