package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// reciteEntry replaces one citation of a statement: the one carrying
// old_url and old_quote becomes new_url, new_locator, new_quote. It is how
// an agent hands over the citations widen could not do on its own: a
// quote re-pointed from a PDF the extractor fuses to the HTML edition, a
// reference-only page swapped for the statute it summarises, a dead URL
// replaced. Fields left empty keep the citation's current value.
type reciteEntry struct {
	StatementKey string `json:"statement_key"`
	OldURL       string `json:"old_url"`
	OldQuote     string `json:"old_quote"`
	NewURL       string `json:"new_url,omitempty"`
	NewPublisher string `json:"new_publisher,omitempty"`
	NewLocator   string `json:"new_locator,omitempty"`
	NewQuote     string `json:"new_quote"`
	// From and To name the passage instead of NewQuote: the quote runs
	// from the first occurrence of From to the end of the next occurrence
	// of To in the page text. Short anchors, so a subsection can be named
	// without retyping it.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	Note string `json:"note"`
}

// passageBetween cuts the whitespace-normalized page text from the first
// occurrence of from to the end of the following occurrence of to.
func passageBetween(text, from, to string) (string, bool) {
	hay := strings.Join(strings.Fields(text), " ")
	from, to = strings.Join(strings.Fields(from), " "), strings.Join(strings.Fields(to), " ")
	i := strings.Index(hay, from)
	if i < 0 {
		return "", false
	}
	j := strings.Index(hay[i:], to)
	if j < 0 {
		return "", false
	}
	return hay[i : i+j+len(to)], true
}

// recite turns a file of reciteEntry into a cmd/propose file, one
// whole-statement edit per statement under agent-pass:widen-quote. Every
// new quote is fetched and confirmed verbatim here; an entry whose quote
// is not on the page is refused, so nothing unverifiable reaches check.
//
//	triage recite <entries.json> > proposals.json
func recite(ctx context.Context, pg *store.PG, tb *drafting.Toolbelt, path string) {
	raw, err := os.ReadFile(path) // #nosec G703 -- the file named on the command line is the job
	if err != nil {
		fatal(err)
	}
	var entries []reciteEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	type proposal struct {
		StatementKey string                   `json:"statement_key"`
		Reason       string                   `json:"reason"`
		Proposed     *store.ProposedStatement `json:"proposed"`
		Evidence     map[string]any           `json:"evidence"`
	}
	byKey := map[string]*proposal{}
	var order []string
	problems := 0
	for i, e := range entries {
		url := e.NewURL
		if url == "" {
			url = e.OldURL
		}
		out, err := tb.FetchSource(ctx, drafting.FetchSourceInput{URL: url})
		if err != nil {
			problems++
			fmt.Fprintf(os.Stderr, "entry %d: %v\n", i+1, err)
			continue
		}
		if e.NewQuote == "" && e.From != "" {
			q, ok := passageBetween(out.Text, e.From, e.To)
			if !ok {
				problems++
				fmt.Fprintf(os.Stderr, "entry %d: anchors not found in %s\n", i+1, url)
				continue
			}
			e.NewQuote = q
		}
		if !drafting.QuoteAppearsIn(out.Text, e.NewQuote) {
			problems++
			fmt.Fprintf(os.Stderr, "entry %d: new quote is not verbatim in %s\n", i+1, url)
			continue
		}
		p, ok := byKey[e.StatementKey]
		if !ok {
			st, err := pg.StatementByKey(ctx, e.StatementKey)
			if err != nil {
				fatal(fmt.Errorf("entry %d: statement %s: %w", i+1, e.StatementKey, err))
			}
			p = &proposal{StatementKey: e.StatementKey, Reason: "agent-pass:widen-quote", Proposed: &st, Evidence: map[string]any{}}
			byKey[e.StatementKey] = p
			order = append(order, e.StatementKey)
		}
		replaced := false
		for ci := range p.Proposed.Citations {
			c := &p.Proposed.Citations[ci]
			if c.URL != e.OldURL || strings.TrimSpace(c.Quote) != strings.TrimSpace(e.OldQuote) {
				continue
			}
			c.URL = url
			if e.NewPublisher != "" {
				c.Publisher = e.NewPublisher
			}
			if e.NewLocator != "" {
				c.Locator = e.NewLocator
			}
			c.Quote = e.NewQuote
			c.Checked = out.Via == ""
			c.CheckedVia = ""
			if c.Checked {
				c.CheckedVia = "direct fetch"
			}
			replaced = true
		}
		if !replaced {
			problems++
			fmt.Fprintf(os.Stderr, "entry %d: the statement carries no citation of %s with that quote\n", i+1, e.OldURL)
			continue
		}
		note, _ := p.Evidence["note"].(string)
		p.Evidence["note"] = strings.TrimSpace(note + " " + e.Note)
		p.Evidence["old_quote"], p.Evidence["new_quote"], p.Evidence["source_url"] = e.OldQuote, e.NewQuote, url
	}
	out := make([]*proposal, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	emit(out)
	fmt.Fprintf(os.Stderr, "%d proposals from %d entries; %d entries refused\n", len(out), len(entries), problems)
	if problems > 0 {
		os.Exit(1)
	}
}
