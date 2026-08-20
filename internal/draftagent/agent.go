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
	"io"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/nazanindev/defensiverenting/internal/drafting"
)

// Options configures a single drafting run.
type Options struct {
	// JurisdictionSlug scopes the page: a city, a state, or "united-states"
	// for a nationwide page.
	JurisdictionSlug string
	TopicSlug        string
	TopicName        string
	// Language defaults to "en". "es" asks the agent to translate the
	// existing English playbook (see the system prompt) rather than
	// research independently — see voice.Supported for the full set.
	Language string
	Model    string                        // defaults to claude-opus-4-8
	MaxSteps int                           // defaults to 30
	Log      func(format string, a ...any) // progress sink; nil = discard
	// Transcript, when non-nil, receives one JSON line per run event: the run
	// inputs, every raw model response, every tool result, and the final
	// Report. Writes are not synchronized — give each run its own writer.
	Transcript io.Writer
}

// Report summarizes what a drafting run spent and produced, so runs can be
// compared across models on cost per accepted draft. It is meaningful even
// when Run returns an error: counters cover everything up to the failure.
type Report struct {
	Model string `json:"model"`
	// Steps is the number of model calls made, including pause_turn
	// continuations for server-side web_search.
	Steps int  `json:"steps"`
	Saved bool `json:"saved"`
	// ToolCalls counts client tool dispatches; Rejections counts the subset
	// returned to the model as errors (toolbelt rejections, invalid
	// arguments, unknown tools), each of which costs the model a retry.
	ToolCalls  int           `json:"tool_calls"`
	Rejections int           `json:"rejections"`
	Duration   time.Duration `json:"duration_ns"`
	Usage      TokenUsage    `json:"usage"`
}

// TokenUsage accumulates billed token and server-tool counts across every
// model call in a run.
type TokenUsage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	WebSearchRequests   int64 `json:"web_search_requests"`
}

func (u *TokenUsage) add(usage anthropic.Usage) {
	u.InputTokens += usage.InputTokens
	u.OutputTokens += usage.OutputTokens
	u.CacheReadTokens += usage.CacheReadInputTokens
	u.CacheCreationTokens += usage.CacheCreationInputTokens
	u.WebSearchRequests += usage.ServerToolUse.WebSearchRequests
}

// Run drives the drafting loop to completion. The error is nil once a draft
// has been saved, and set if the model finished, refused, or ran out of steps
// without saving; the Report is valid either way. Requires ANTHROPIC_API_KEY
// in the environment.
func Run(ctx context.Context, tb *drafting.Toolbelt, opts Options) (report Report, err error) {
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

	report.Model = model
	tr := &transcript{w: opts.Transcript, logf: logf}
	tr.emit(event{Type: "run", Run: &runInfo{
		Jurisdiction: opts.JurisdictionSlug,
		Topic:        opts.TopicSlug,
		Language:     opts.Language,
		Model:        model,
		MaxSteps:     maxSteps,
	}})
	start := time.Now()
	defer func() {
		report.Duration = time.Since(start)
		done := event{Type: "done", Report: &report}
		if err != nil {
			done.Error = err.Error()
		}
		tr.emit(done)
	}()

	client := anthropic.NewClient()
	params := anthropic.MessageNewParams{
		Model:        anthropic.Model(model),
		MaxTokens:    8192,
		Thinking:     anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{}},
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortHigh},
		System:       []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:        toolDefs(),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt(opts.JurisdictionSlug, opts.TopicSlug, opts.TopicName, opts.Language))),
		},
	}

	for step := 1; step <= maxSteps; step++ {
		resp, cerr := client.Messages.New(ctx, params)
		if cerr != nil {
			return report, fmt.Errorf("model call step %d: %w", step, cerr)
		}
		report.Steps = step
		report.Usage.add(resp.Usage)
		tr.message(step, resp)

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
					return report, fmt.Errorf("tool %q: %w", block.Name, fatal)
				}
				report.ToolCalls++
				if oc.isError {
					report.Rejections++
					logf("  ✗ %s", oc.content)
				} else if block.Name == "save_draft_playbook" {
					report.Saved = true
				}
				tr.emit(event{Type: "tool_result", Step: step, Tool: block.Name, IsError: oc.isError, Content: oc.content})
				results = append(results, anthropic.NewToolResultBlock(block.ID, oc.content, oc.isError))
			}
			params.Messages = append(params.Messages, anthropic.NewUserMessage(results...))
		case anthropic.StopReasonPauseTurn:
			// Server-side tool (web_search) loop paused; re-send to continue.
			continue
		default:
			if resp.StopReason == anthropic.StopReasonRefusal {
				return report, fmt.Errorf("model refused: %v", resp.StopDetails)
			}
			if report.Saved {
				return report, nil
			}
			return report, fmt.Errorf("model finished without saving a draft (stop_reason=%s)", resp.StopReason)
		}
	}
	return report, fmt.Errorf("hit max-steps (%d) without saving a draft", maxSteps)
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
