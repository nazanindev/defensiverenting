package main

import (
	"html/template"
	"strings"
)

// wordDiff renders the change from old to new as one passage: words only in
// old struck through, words only in new marked, the rest plain. The reviewer
// reads the edit in place instead of comparing two columns. Every segment is
// escaped.
func wordDiff(old, new string) template.HTML {
	a, b := strings.Fields(old), strings.Fields(new)
	var out strings.Builder
	flush := func(tag string, words []string) {
		if len(words) == 0 {
			return
		}
		if out.Len() > 0 {
			out.WriteByte(' ')
		}
		if tag != "" {
			out.WriteString("<" + tag + ">")
		}
		out.WriteString(template.HTMLEscapeString(strings.Join(words, " ")))
		if tag != "" {
			out.WriteString("</" + tag + ">")
		}
	}
	var same, del, ins []string
	for _, op := range diffOps(a, b) {
		switch op.kind {
		case ' ':
			flush("del", del)
			flush("ins", ins)
			del, ins = nil, nil
			same = append(same, op.word)
		case '-':
			flush("", same)
			same = nil
			del = append(del, op.word)
		case '+':
			flush("", same)
			same = nil
			ins = append(ins, op.word)
		}
	}
	flush("", same)
	flush("del", del)
	flush("ins", ins)
	return template.HTML(out.String()) //nolint:gosec // every segment is escaped above
}

type diffOp struct {
	kind byte // ' ' kept, '-' only in old, '+' only in new
	word string
}

// diffOps is the longest-common-subsequence edit script over words.
func diffOps(a, b []string) []diffOp {
	n, m := len(a), len(b)
	// lcs[i][j] is the LCS length of a[i:] and b[j:].
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', b[j]})
	}
	return ops
}
