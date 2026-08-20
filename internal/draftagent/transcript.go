package draftagent

import (
	"encoding/json"
	"io"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// transcript appends one JSON line per run event to a writer, so a run's
// model-visible conversation and outcome can be audited later or compared
// across models. A nil writer disables it. A marshal or write failure is
// reported once through logf and the remaining events are dropped: losing the
// transcript must not fail a paid drafting run.
type transcript struct {
	w    io.Writer
	logf func(format string, a ...any)
	dead bool
}

// event is one transcript line. Type selects which optional fields are set:
// "run" carries run; "assistant" carries step and message (the raw API
// response, including usage and stop_reason); "tool_result" carries step,
// tool, is_error, and content; "done" carries report and, on failure, error.
type event struct {
	TS      string          `json:"ts"`
	Type    string          `json:"type"`
	Step    int             `json:"step,omitempty"`
	Run     *runInfo        `json:"run,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
	Tool    string          `json:"tool,omitempty"`
	IsError bool            `json:"is_error,omitempty"`
	Content string          `json:"content,omitempty"`
	Report  *Report         `json:"report,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// runInfo pins the inputs that, together with the code version, determine the
// run's system and user prompts.
type runInfo struct {
	Jurisdiction string `json:"jurisdiction"`
	Topic        string `json:"topic"`
	Language     string `json:"language"`
	Model        string `json:"model"`
	MaxSteps     int    `json:"max_steps"`
}

// message records one raw API response; marshaling is skipped entirely when
// the transcript is disabled.
func (t *transcript) message(step int, resp *anthropic.Message) {
	if t.w == nil || t.dead {
		return
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.fail(err)
		return
	}
	t.emit(event{Type: "assistant", Step: step, Message: b})
}

func (t *transcript) emit(ev event) {
	if t.w == nil || t.dead {
		return
	}
	ev.TS = time.Now().UTC().Format(time.RFC3339)
	b, err := json.Marshal(ev)
	if err == nil {
		_, err = t.w.Write(append(b, '\n'))
	}
	if err != nil {
		t.fail(err)
	}
}

func (t *transcript) fail(err error) {
	t.dead = true
	t.logf("transcript disabled: %v", err)
}
