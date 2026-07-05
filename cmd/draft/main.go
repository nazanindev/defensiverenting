// Command draft runs the AI drafting worker: given a city and topic, it drives
// an Anthropic tool-use loop where Claude researches authoritative primary
// sources (hosted web_search + our find_sources), reads them through our
// fetch_source (so the verbatim guardrail can verify quotes), and writes a
// status="draft" playbook via save_draft_playbook. Pure Go — no `claude` CLI.
//
// Auth: ANTHROPIC_API_KEY. DB: DATABASE_URL (or -db). Model: claude-opus-4-8.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

func main() {
	log.SetFlags(0)
	city := flag.String("city", "", "city slug, e.g. boston (required)")
	topic := flag.String("topic", "", "topic slug, e.g. security-deposits (required)")
	topicName := flag.String("topic-name", "", "topic display name (defaults to a title-cased slug)")
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	modelFlag := flag.String("model", string(anthropic.ModelClaudeOpus4_8), "Anthropic model id")
	maxSteps := flag.Int("max-steps", 30, "max tool-use turns before giving up")
	flag.Parse()

	if *city == "" || *topic == "" {
		log.Fatal("draft: -city and -topic are required")
	}
	if *dsn == "" {
		log.Fatal("draft: DATABASE_URL (or -db) is required")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		log.Fatal("draft: ANTHROPIC_API_KEY is required")
	}

	ctx := context.Background()
	pg, err := store.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("draft: connect db: %v", err)
	}
	defer pg.Close()

	tb := drafting.New(pg)
	client := anthropic.NewClient()

	params := anthropic.MessageNewParams{
		Model:        anthropic.Model(*modelFlag),
		MaxTokens:    8192,
		Thinking:     anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{}},
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortHigh},
		System:       []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:        toolDefs(),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt(*city, *topic, *topicName))),
		},
	}

	saved := false
	for step := 1; step <= *maxSteps; step++ {
		resp, err := client.Messages.New(ctx, params)
		if err != nil {
			log.Fatalf("draft: model call failed on step %d: %v", step, err)
		}

		// Surface any narration text to stderr so the run is observable.
		for _, block := range resp.Content {
			if block.Type == "text" && block.Text != "" {
				fmt.Fprintf(os.Stderr, "· %s\n", block.Text)
			}
		}

		params.Messages = append(params.Messages, resp.ToParam())

		switch resp.StopReason {
		case anthropic.StopReasonToolUse:
			var results []anthropic.ContentBlockParamUnion
			for _, block := range resp.Content {
				if block.Type != "tool_use" {
					continue
				}
				fmt.Fprintf(os.Stderr, "→ %s\n", block.Name)
				outcome, fatal := execTool(ctx, tb, block.Name, block.Input)
				if fatal != nil {
					log.Fatalf("draft: tool %q failed: %v", block.Name, fatal)
				}
				if outcome.isError {
					fmt.Fprintf(os.Stderr, "  ✗ %s\n", outcome.content)
				} else if block.Name == "save_draft_playbook" {
					saved = true
				}
				results = append(results, anthropic.NewToolResultBlock(block.ID, outcome.content, outcome.isError))
			}
			params.Messages = append(params.Messages, anthropic.NewUserMessage(results...))
		case anthropic.StopReasonPauseTurn:
			// Server-side tool (web_search) loop paused; re-send to continue.
			continue
		default:
			// end_turn / max_tokens / refusal — the model is done.
			if resp.StopReason == anthropic.StopReasonRefusal {
				log.Fatalf("draft: model refused: %v", resp.StopDetails)
			}
			if saved {
				fmt.Printf("✓ draft saved for %s/%s — open it in the authoring tool to verify and publish.\n", *city, *topic)
				return
			}
			log.Fatalf("draft: model finished without saving a draft (stop_reason=%s)", resp.StopReason)
		}
	}
	log.Fatalf("draft: hit max-steps (%d) without saving a draft", *maxSteps)
}

type outcome struct {
	content string
	isError bool
}

// execTool dispatches a model tool call to the toolbelt. A *RejectionError comes
// back as an is_error tool result the model can read and fix; any other error is
// fatal to the run.
func execTool(ctx context.Context, tb *drafting.Toolbelt, name string, input json.RawMessage) (outcome, error) {
	switch name {
	case "find_sources":
		return call(ctx, input, tb.FindSources)
	case "fetch_source":
		return call(ctx, input, tb.FetchSource)
	case "save_draft_playbook":
		return call(ctx, input, tb.SaveDraft)
	case "list_jurisdictions":
		return call(ctx, input, func(ctx context.Context, _ struct{}) (drafting.ListJurisdictionsOutput, error) {
			return tb.ListJurisdictions(ctx)
		})
	case "list_topics":
		return call(ctx, input, tb.ListTopics)
	case "get_playbook":
		return call(ctx, input, tb.GetPlaybook)
	default:
		return outcome{content: fmt.Sprintf("unknown tool %q", name), isError: true}, nil
	}
}

func call[In, Out any](ctx context.Context, raw json.RawMessage, fn func(context.Context, In) (Out, error)) (outcome, error) {
	var in In
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return outcome{content: fmt.Sprintf("invalid arguments: %v", err), isError: true}, nil
		}
	}
	out, err := fn(ctx, in)
	var re *drafting.RejectionError
	if errors.As(err, &re) {
		return outcome{content: re.Msg, isError: true}, nil
	}
	if err != nil {
		return outcome{}, err
	}
	b, err := json.Marshal(out)
	if err != nil {
		return outcome{}, err
	}
	return outcome{content: string(b)}, nil
}
