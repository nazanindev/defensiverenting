// Package draftagent runs the AI drafting loop: it drives an Anthropic tool-use
// conversation in which Claude researches authoritative primary sources (hosted
// web_search + our find_sources), reads them through fetch_source (so the
// verbatim guardrail can verify quotes), and writes a status="draft" playbook.
//
// It is front-end-agnostic: cmd/draft calls Run from the CLI, and the authoring
// server calls it in a goroutine behind a "Generate draft" button. Both pass a
// *drafting.Toolbelt, so the same guardrailed tools back every path.
package draftagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/nazanindev/defensiverenting/internal/drafting"
)

// Options configures a single drafting run.
type Options struct {
	CitySlug  string
	TopicSlug string
	TopicName string
	// Language defaults to "en". "es" asks the agent to translate the
	// existing English playbook (see the system prompt) rather than
	// research independently — see voice.Supported for the full set.
	Language string
	Model    string                        // defaults to claude-opus-4-8
	MaxSteps int                           // defaults to 30
	Log      func(format string, a ...any) // progress sink; nil = discard
}

// Run drives the drafting loop to completion. It returns nil once a draft has
// been saved, or an error if the model finished, refused, or ran out of steps
// without saving. Requires ANTHROPIC_API_KEY in the environment.
func Run(ctx context.Context, tb *drafting.Toolbelt, opts Options) error {
	logf := opts.Log
	if logf == nil {
		logf = func(string, ...any) {}
	}
	model := opts.Model
	if model == "" {
		model = string(anthropic.ModelClaudeOpus4_8)
	}
	maxSteps := opts.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 30
	}

	client := anthropic.NewClient()
	params := anthropic.MessageNewParams{
		Model:        anthropic.Model(model),
		MaxTokens:    8192,
		Thinking:     anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{}},
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortHigh},
		System:       []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:        toolDefs(),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt(opts.CitySlug, opts.TopicSlug, opts.TopicName, opts.Language))),
		},
	}

	saved := false
	for step := 1; step <= maxSteps; step++ {
		resp, err := client.Messages.New(ctx, params)
		if err != nil {
			return fmt.Errorf("model call step %d: %w", step, err)
		}

		for _, block := range resp.Content {
			if block.Type == "text" && block.Text != "" {
				logf("· %s", block.Text)
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
				logf("→ %s", block.Name)
				oc, fatal := execTool(ctx, tb, block.Name, block.Input)
				if fatal != nil {
					return fmt.Errorf("tool %q: %w", block.Name, fatal)
				}
				if oc.isError {
					logf("  ✗ %s", oc.content)
				} else if block.Name == "save_draft_playbook" {
					saved = true
				}
				results = append(results, anthropic.NewToolResultBlock(block.ID, oc.content, oc.isError))
			}
			params.Messages = append(params.Messages, anthropic.NewUserMessage(results...))
		case anthropic.StopReasonPauseTurn:
			// Server-side tool (web_search) loop paused; re-send to continue.
			continue
		default:
			if resp.StopReason == anthropic.StopReasonRefusal {
				return fmt.Errorf("model refused: %v", resp.StopDetails)
			}
			if saved {
				return nil
			}
			return fmt.Errorf("model finished without saving a draft (stop_reason=%s)", resp.StopReason)
		}
	}
	return fmt.Errorf("hit max-steps (%d) without saving a draft", maxSteps)
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
