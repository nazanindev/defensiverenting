package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// Rule flag (ADR-021 D3, second rule): a reviewer flag on a draft page is a
// doubt the drafting agent had about a claim. A second, independent model
// reads the claim, the doubt, and the cited sections as fetched now, and
// says whether the cited text itself answers the doubt. It may only answer
// "stands" by quoting the passage that answers it, and that passage must
// appear verbatim in the fetched text or the answer is thrown away. What it
// cannot settle from the cited sections stays pending for a person.
//
// The model never sees the drafter's reasoning, only what a reviewer would
// see on the card. It has no tools: it cannot search, fetch, or cite
// anything the statement does not already cite. A doubt whose answer lies
// outside the cited sources is a person's question by design.

const flagModel = "claude-opus-5"

// flagSourceCap is the most of one fetched source the model is shown. A
// statute section rarely exceeds it; a whole chapter does, and the model is
// told when it is looking at a cut, so it can leave the item rather than
// guess from half a document.
const flagSourceCap = 80_000

// flagPassageCap bounds the passage a "stands" answer quotes: enough for a
// subsection, not enough to paste the document back.
const flagPassageCap = 150

const flagSystem = `You are the second reader on a legal information site for renters. Statements on the site are short claims about renter law in one place, each backed by a verbatim quote from a primary source. The first reader, the drafting model, flagged some statements with a doubt. You decide whether the cited sources, as fetched today, answer that doubt.

You have exactly one job and one tool. Read the statement, the doubt, and the fetched text of each cited source. Then call decide_flag once.

Answer "stands" only when all of these hold:
- The cited text itself settles the doubt. Not your general knowledge of the law, not what is probably true, only the text in front of you.
- You can quote the passage that settles it, verbatim, from the fetched text. Copy it exactly; do not paraphrase, do not fix punctuation, do not merge two places into one quote.
- The statement, as written, is consistent with that passage. If the passage shows the statement is wrong, incomplete in a way that matters to a renter, or narrower than the statement claims, answer "leave".

Answer "leave" in every other case, including: the doubt asks about something the cited sources do not cover; the fetched text is a cut or a shell and the answer may be outside it; the passage you would quote is not verbatim in the text; the statement needs an edit. "Leave" costs nothing; a person will read it. A wrong "stands" puts a wrong claim in front of a renter.

Your reason is one or two plain sentences for the person auditing you. Say what the doubt was and what the passage settles. No hedging language.`

type flagDecision struct {
	Verdict string `json:"verdict"`
	Passage string `json:"passage"`
	Reason  string `json:"reason"`
}

func flagTool() anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
		Name:        "decide_flag",
		Description: anthropic.String("Record the decision on this flag. Call it exactly once."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"verdict": map[string]any{"type": "string", "enum": []string{"stands", "leave"}, "description": "stands: the cited text answers the doubt and the statement is consistent with it. leave: anything else."},
				"passage": map[string]any{"type": "string", "description": "For stands: the passage from the fetched text that settles the doubt, copied verbatim. For leave: empty."},
				"reason":  map[string]any{"type": "string", "description": "One or two plain sentences for the auditor."},
			},
			Required: []string{"verdict", "reason"},
		},
	}}
}

// flagSource is one cited source as shown to the model.
type flagSource struct {
	URL, Publisher, Locator, Quote string
	Text                           string // fetched, possibly cut to flagSourceCap
	Cut                            bool
	Err                            string // why there is no text
}

func flagPrompt(st store.CitedStatement, note string, srcs []flagSource, page store.PlaybookWithStatements) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Page: %s, %s (%s)\n\n", page.Jurisdiction.Name, page.Topic.Name, page.Title)
	fmt.Fprintf(&b, "Statement:\n%s\n\n", st.BodyMD)
	fmt.Fprintf(&b, "The first reader's doubt:\n%s\n\n", note)
	for i, s := range srcs {
		fmt.Fprintf(&b, "=== Cited source %d: %s\nURL: %s\nLocator: %s\nQuote on the statement: %s\n", i+1, s.Publisher, s.URL, orNone(s.Locator), orNone(s.Quote))
		switch {
		case s.Err != "":
			fmt.Fprintf(&b, "Fetched text: none. %s\n\n", s.Err)
		case s.Cut:
			fmt.Fprintf(&b, "Fetched text (CUT at %d characters; the document continues past this):\n%s\n\n", flagSourceCap, s.Text)
		default:
			fmt.Fprintf(&b, "Fetched text:\n%s\n\n", s.Text)
		}
	}
	b.WriteString("Decide with decide_flag.")
	return b.String()
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(none)"
	}
	return s
}

// flagVerdict applies the invariants to what the model answered. A "stands"
// survives only when its passage is verbatim in one of the fetched texts and
// short enough to be a passage. Returns the accepted decision, or why the
// item stays pending.
func flagVerdict(d flagDecision, srcs []flagSource) (stands bool, why string) {
	d.Verdict = strings.ToLower(strings.TrimSpace(d.Verdict))
	d.Reason = strings.TrimSpace(d.Reason)
	if d.Verdict != "stands" {
		if d.Reason == "" {
			return false, "left by the second reader"
		}
		return false, "left by the second reader: " + d.Reason
	}
	if d.Reason == "" {
		return false, "second reader gave no reason; left"
	}
	passage := strings.TrimSpace(d.Passage)
	if passage == "" {
		return false, "second reader said stands without quoting the passage; left"
	}
	if n := len(strings.Fields(passage)); n > flagPassageCap {
		return false, fmt.Sprintf("second reader's passage runs %d words, over the %d-word cap; left", n, flagPassageCap)
	}
	for _, s := range srcs {
		if s.Text != "" && drafting.QuoteAppearsIn(s.Text, passage) {
			return true, ""
		}
	}
	return false, "second reader's passage is not verbatim in any cited source; left"
}

func decideFlag(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("decide flag", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the decisions; default prints them (the model is called either way)")
	limit := fs.Int("limit", 0, "stop after this many items read (0 = all)")
	model := fs.String("model", flagModel, "the second reader")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fatal(fmt.Errorf("ANTHROPIC_API_KEY is not set; the second reader is a model call"))
	}
	pending, err := pg.ListProposalsByReason(ctx, "pending", "note")
	if err != nil {
		fatal(err)
	}
	client := anthropic.NewClient()
	fetched := map[string]flagSource{}
	fetch := func(url string) flagSource {
		if s, ok := fetched[url]; ok {
			return s
		}
		var s flagSource
		rc, err := drafting.FetchExtract(url)
		switch {
		case err != nil:
			s.Err = "The source could not be fetched: " + err.Error()
		case !rc.Readable():
			s.Err = "The source answered with no readable text (" + rc.Describe() + ")."
		default:
			s.Text = rc.Text
			if len(s.Text) > flagSourceCap {
				s.Text, s.Cut = s.Text[:flagSourceCap], true
			}
		}
		fetched[url] = s
		return s
	}
	pages := map[int64]store.PlaybookWithStatements{}
	seen, stood, left := 0, 0, 0
	for _, p := range pending {
		if p.TargetStatus != "draft" || !p.OnPage() {
			continue
		}
		if *limit > 0 && seen >= *limit {
			break
		}
		var ev store.ReviewerFlagEvidence
		if json.Unmarshal(p.Evidence, &ev) != nil || strings.TrimSpace(ev.Note) == "" {
			continue
		}
		pw, ok := pages[p.TargetPlaybookID]
		if !ok {
			if pw, err = pg.AuthorGetPlaybook(ctx, p.TargetPlaybookID); err != nil {
				fatal(err)
			}
			pages[p.TargetPlaybookID] = pw
		}
		var st store.CitedStatement
		for _, s := range pw.Statements {
			if s.Key == p.StatementKey {
				st = s
			}
		}
		if st.Key == "" {
			continue
		}
		var srcs []flagSource
		for _, c := range st.Citations {
			if c.SourceKind == "editorial" {
				continue
			}
			s := fetch(c.SourceURL)
			s.URL, s.Publisher, s.Locator, s.Quote = c.SourceURL, c.Publisher, c.Locator, c.Quote
			srcs = append(srcs, s)
		}
		seen++
		fmt.Printf("#%d %s · %s · statement %d\n    doubt: %s\n", p.ID, p.JurisdictionName, p.TopicName, p.Position, truncate(ev.Note, 160))
		if len(srcs) == 0 {
			left++
			fmt.Println("    left pending: the statement cites no primary source to read")
			continue
		}
		d, err := askFlag(ctx, client, *model, flagPrompt(st, ev.Note, srcs, pw))
		if err != nil {
			left++
			fmt.Printf("    left pending: %v\n", err)
			continue
		}
		stands, why := flagVerdict(d, srcs)
		if !stands {
			left++
			fmt.Printf("    left pending: %s\n", why)
			continue
		}
		fmt.Printf("    stands: %s\n    passage: %s\n", d.Reason, truncate(strings.TrimSpace(d.Passage), 200))
		if !*apply {
			stood++
			continue
		}
		note := fmt.Sprintf("Review agent, rule flag (%s): stands as written. %s Passage: “%s”", *model, d.Reason, strings.TrimSpace(d.Passage))
		if err := pg.DecideProposal(ctx, p.ID, "rejected", store.ActorReviewAgent, note, nil); err != nil {
			left++
			fmt.Printf("    left pending: %v\n", err)
			continue
		}
		stood++
	}
	if *apply {
		fmt.Printf("%d flags read; %d closed as stands by %s; %d left for a person\n", seen, stood, store.ActorReviewAgent, left)
	} else {
		fmt.Printf("%d flags read; %d would close as stands; %d left for a person; nothing written (add -apply)\n", seen, stood, left)
	}
}

// askFlag makes the one model call for a flag and returns the decision it
// recorded through the tool. No tool call, a refusal, or a cut-off answer
// all read as "leave".
func askFlag(ctx context.Context, client anthropic.Client, model, prompt string) (flagDecision, error) {
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:        anthropic.Model(model),
		MaxTokens:    4096,
		Thinking:     anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{}},
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortHigh},
		System:       []anthropic.TextBlockParam{{Text: flagSystem}},
		Tools:        []anthropic.ToolUnionParam{flagTool()},
		Messages:     []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
	})
	if err != nil {
		return flagDecision{}, fmt.Errorf("model call: %w", err)
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return flagDecision{}, fmt.Errorf("model refused")
	}
	for _, block := range resp.Content {
		if v, ok := block.AsAny().(anthropic.ToolUseBlock); ok && v.Name == "decide_flag" {
			var d flagDecision
			if err := json.Unmarshal([]byte(v.JSON.Input.Raw()), &d); err != nil {
				return flagDecision{}, fmt.Errorf("decision not readable: %w", err)
			}
			return d, nil
		}
	}
	return flagDecision{Verdict: "leave", Reason: "the second reader gave no decision"}, nil
}
