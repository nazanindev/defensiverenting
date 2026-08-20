package draftagent

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Transcripts are JSONL: one self-describing object per line, each stamped
// with ts and type, so jq can mine them without knowing the event order.
func TestTranscriptEmitsOneJSONObjectPerLine(t *testing.T) {
	var buf strings.Builder
	tr := &transcript{w: &buf, logf: func(string, ...any) {}}

	tr.emit(event{Type: "run", Run: &runInfo{Jurisdiction: "boston", Topic: "eviction-defense", Model: "m", MaxSteps: 30}})
	tr.emit(event{Type: "tool_result", Step: 3, Tool: "fetch_source", IsError: true, Content: "nope"})
	report := Report{Model: "m", Steps: 3, Saved: true}
	tr.emit(event{Type: "done", Report: &report})

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%s", len(lines), buf.String())
	}
	for i, line := range lines {
		var ev struct {
			TS   string `json:"ts"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("line %d is not JSON: %v", i+1, err)
		}
		if ev.TS == "" || ev.Type == "" {
			t.Errorf("line %d missing ts or type: %s", i+1, line)
		}
	}
	if !strings.Contains(lines[1], `"is_error":true`) {
		t.Errorf("tool_result line lost is_error: %s", lines[1])
	}
	if !strings.Contains(lines[2], `"saved":true`) {
		t.Errorf("done line lost the report: %s", lines[2])
	}
}

// A nil writer must be a no-op: the authoring server passes no Transcript.
func TestTranscriptNilWriterIsNoop(t *testing.T) {
	tr := &transcript{w: nil, logf: func(format string, a ...any) {
		t.Errorf("logf called on nil-writer transcript: "+format, a...)
	}}
	tr.emit(event{Type: "run"})
	tr.message(1, nil)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// A write failure reports once and drops the rest: transcripts must never
// fail a paid drafting run, and must not spam the progress log either.
func TestTranscriptWriteFailureDisablesAndLogsOnce(t *testing.T) {
	var logged []string
	tr := &transcript{w: failingWriter{}, logf: func(format string, a ...any) {
		logged = append(logged, format)
	}}
	tr.emit(event{Type: "run"})
	tr.emit(event{Type: "done"})
	if len(logged) != 1 {
		t.Fatalf("logf called %d times, want 1 (%q)", len(logged), logged)
	}
}
