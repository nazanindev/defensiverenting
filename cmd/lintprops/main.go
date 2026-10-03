// lintprops runs the voice lint that `triage check` runs on a proposals file,
// without a database. Review agents loop on it before handing a file back,
// so lint problems are fixed by the agent that wrote the text, and the
// filing step (triage check, then propose) only refuses for source reasons.
//
// The optional second file is the work-items input the agent was given
// (statement_key + body_md per item). With it, a replacement is also held to
// voice.HarderThan against the body it replaces, as triage check does.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/voice"
)

type statement struct {
	BodyMD string `json:"body_md"`
}

type proposed struct {
	Action    string      `json:"action"`
	BodyMD    string      `json:"body_md"`
	Followers []statement `json:"followers"`
}

type entry struct {
	StatementKey string    `json:"statement_key"`
	Proposed     *proposed `json:"proposed"`
}

type workItem struct {
	StatementKey string `json:"statement_key"`
	Key          string `json:"key"` // unstamped listings name it key
	BodyMD       string `json:"body_md"`
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 || len(os.Args) > 3 {
		log.Fatal("usage: lintprops <props.json> [<work-items.json>]")
	}
	var entries []entry
	readJSON(os.Args[1], &entries)
	current := map[string]string{}
	if len(os.Args) == 3 {
		var items []workItem
		readJSON(os.Args[2], &items)
		for _, it := range items {
			k := it.StatementKey
			if k == "" {
				k = it.Key
			}
			current[strings.ToLower(k)] = it.BodyMD
		}
	}
	const lang = "en"
	problems := 0
	report := func(where string, msgs ...string) {
		problems += len(msgs)
		fmt.Printf("%s:\n  - %s\n", where, strings.Join(msgs, "\n  - "))
	}
	for i, e := range entries {
		p := e.Proposed
		if p == nil || p.Action == "remove" || p.Action == "reorder" {
			continue
		}
		where := fmt.Sprintf("entry %d", i+1)
		old, known := current[strings.ToLower(e.StatementKey)]
		if !known || p.BodyMD != old {
			if v := voice.LintAll(lang, map[string]string{"body_md": p.BodyMD}); len(v) > 0 {
				report(where+": voice lint", v...)
			}
			if known {
				if why := voice.HarderThan(lang, old, p.BodyMD); why != "" {
					report(where, why)
				}
			}
		}
		for fi, f := range p.Followers {
			if v := voice.LintAll(lang, map[string]string{"body_md": f.BodyMD}); len(v) > 0 {
				report(fmt.Sprintf("%s follower %d: voice lint", where, fi+1), v...)
			}
		}
	}
	fmt.Printf("%d entries, %d lint problems\n", len(entries), problems)
	if problems > 0 {
		os.Exit(1)
	}
}

func readJSON(path string, v any) {
	raw, err := os.ReadFile(path) // #nosec G703 -- the file named on the command line is the job
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		log.Fatalf("decode %s: %v", path, err)
	}
}
