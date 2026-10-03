// dumpsrc prints the readable text of one source URL, read the way the quote
// checker reads it (drafting.FetchExtract: direct fetch, then a headless
// render, never an archive snapshot). Review agents dump a source once and
// copy quotes from the dump by script, so a quote they file is verbatim in
// the same text the checker will compare it against. The receipt line goes
// to stderr so stdout is the text alone.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nazanindev/defensiverenting/internal/drafting"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) != 2 {
		log.Fatal("usage: dumpsrc <url> > text.txt")
	}
	rc, err := drafting.FetchExtract(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "tier=%s extractor=%s chars=%d thin=%t\n", rc.Tier, rc.Extractor, rc.Chars, rc.Thin)
	fmt.Print(rc.Text)
}
