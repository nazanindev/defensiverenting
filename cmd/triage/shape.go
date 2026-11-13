package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// topicShape prints the four parts of a topic's shape (ADR-036 D4). It
// writes nothing.
//
//	triage shape                 one line per topic: which parts are filled
//	triage shape <topic>         the topic's whole shape
//	triage shape [<topic>] -json the same, for an agent's input file
func topicShape(ctx context.Context, pg *store.PG, args []string) {
	slug := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		slug, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("shape", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "print the shape as JSON")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	shapes, err := pg.TopicShapes(ctx, slug)
	if err != nil {
		fatal(err)
	}
	if *asJSON {
		emit(shapes)
		return
	}
	if slug == "" {
		fmt.Printf("%-26s %s\n", "topic", "not filled")
		for _, s := range shapes {
			missing := strings.Join(s.Missing(), "; ")
			if missing == "" {
				missing = "(all four parts filled)"
			}
			fmt.Printf("%-26s %s\n", s.Slug, missing)
		}
		return
	}

	s := shapes[0]
	fmt.Printf("%s (%s)\n", s.Name, s.Slug)

	fmt.Println("\n1. Title and scope")
	fmt.Printf("  title        %s\n", orDash(s.Scope.TitlePattern))
	if s.Scope.TitlePattern != "" {
		fmt.Printf("  nationwide   %s\n", s.Title(""))
	}
	fmt.Printf("  reader       %s\n", orDash(s.Scope.Reader))
	if s.Scope.IntroOpens != "" {
		fmt.Printf("  intro opens  %s\n", s.Scope.IntroOpens)
	}
	fmt.Printf("  on the page  %s\n", orDash(s.Scope.Covers))
	for _, l := range s.Scope.NotHere {
		fmt.Printf("  not here     %s\n", l)
	}

	fmt.Println("\n2. Tips and warning notes")
	for _, a := range s.Advice {
		use := map[string]string{"every_page": "on every page", "where_it_fits": "where it fits", "never": "never"}[a.Use]
		fmt.Printf("  %-22s %-14s %s\n", a.Slug, use, a.Body)
	}
	if len(s.Advice) == 0 {
		fmt.Println("  -")
	}

	fmt.Println("\n3. Size and order")
	if s.Size.MaxStatements > 0 {
		fmt.Printf("  at most %d statements and %d words on the page, %d words in a statement\n",
			s.Size.MaxStatements, s.Size.MaxWords, s.Size.MaxStatementWords)
	} else {
		fmt.Println("  -")
	}
	for _, l := range s.Size.OrderRules {
		fmt.Printf("  %s\n", l)
	}
	if len(s.Size.Extras) > 0 {
		fmt.Printf("  may also answer, up to %d:\n", s.Size.MaxExtras)
		for _, q := range s.Size.Extras {
			keep := ""
			if q.NeverCut {
				keep = "  (never cut to fit)"
			}
			fmt.Printf("    %-28s %s%s\n", q.Slug, q.Question, keep)
		}
	}
	if s.Size.Example != "" {
		fmt.Printf("  a good statement: %q\n", s.Size.Example)
	}

	fmt.Println("\n4. Must-answer questions")
	for _, st := range s.Stages {
		fmt.Printf("  %s\n", st)
		for _, q := range s.Questions {
			if q.Stage != st {
				continue
			}
			fmt.Printf("    %-28s %-7s %s\n", q.Slug, orDash(q.Answer), q.Question)
			for _, sit := range q.Situations {
				base := ""
				if sit.Base {
					base = "  (base)"
				}
				fmt.Printf("    %-28s         if: %s%s\n", "", sit.Name, base)
			}
		}
	}
	if len(s.Questions) == 0 {
		fmt.Println("  -")
	}
	if m := s.Missing(); len(m) > 0 {
		fmt.Printf("\nNot filled: %s\n", strings.Join(m, "; "))
	}
}

// topicTable prints a topic's must-answer questions across every place
// with a page of the topic (ADR-036 D3, D7). It writes nothing.
//
//	triage table <topic>        every place, one line per question with a number or a hole
//	triage table <topic> -gaps  only the holes: a days question with no number, or a cell nobody checked
//	triage table <topic> -json  the table, for a page or an agent's input file
func topicTable(ctx context.Context, pg *store.PG, args []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fatal(fmt.Errorf("usage: triage table <topic> [-gaps] [-json]"))
	}
	slug, args := args[0], args[1:]
	fs := flag.NewFlagSet("table", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "print the table as JSON")
	gaps := fs.Bool("gaps", false, "print only the holes")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	t, err := pg.TopicTable(ctx, slug)
	if err != nil {
		fatal(err)
	}
	if *asJSON {
		emit(t)
		return
	}
	counts := make([]map[string]int, len(t.Questions))
	holes := make([]int, len(t.Questions))
	for i := range counts {
		counts[i] = map[string]int{}
	}
	for _, r := range t.Rows {
		printed := false
		for i, c := range r.Cells {
			q := t.Questions[i]
			counts[i][c.State]++
			gap := c.Gap(q)
			if gap {
				holes[i]++
			}
			if *gaps && !gap {
				continue
			}
			if !printed {
				fmt.Println(r.Name)
				printed = true
			}
			fmt.Printf("  %-28s %s\n", q.Slug, cellText(c, gap))
		}
	}
	fmt.Printf("\n%-28s %6s %6s %6s %7s %6s   of %d places\n", "question", "number", "words", "no law", "not chk", "holes", len(t.Rows))
	for i, q := range t.Questions {
		fmt.Printf("%-28s %6d %6d %6d %7d %6d   (%s)\n", q.Slug,
			counts[i][store.CellNumber], counts[i][store.CellWords], counts[i][store.CellNoLaw], counts[i][store.CellNotChecked], holes[i], q.Answer)
	}
}

// cellText is one cell in plain words.
func cellText(c store.TableCell, gap bool) string {
	switch c.State {
	case store.CellNumber:
		parts := make([]string, 0, len(c.Values))
		for _, v := range c.Values {
			s := strconv.FormatFloat(v.Value, 'f', -1, 64)
			if v.ValueMax != nil {
				s += "-" + strconv.FormatFloat(*v.ValueMax, 'f', -1, 64)
			}
			s += " " + v.Kind
			if v.Situation != "" {
				s += " " + v.Situation
			}
			if v.Drifted {
				s += " (quote not found at the source)"
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, "; ")
	case store.CellWords:
		if gap {
			return "HOLE: answered on the page, no number stored"
		}
		return "answered in words"
	case store.CellNoLaw:
		return "searched, no law found"
	}
	return "HOLE: not checked"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
