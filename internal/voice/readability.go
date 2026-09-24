package voice

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
)

// Readability rules (2026-09-24). A renter reads a statement once, often on a
// phone in the middle of the problem. What stops them is an unfamiliar word,
// not a long one: "utility" is short and opaque, "refrigerator" is long and
// plain. So the rules measure familiarity, not syllables:
//
//   - UnfamiliarWords flags words outside everyday English. Each must be
//     replaced, or glossed in parentheses right after it.
//   - ReadingScore is the New Dale-Chall score, which grades text by its share
//     of unfamiliar words and its sentence length. A statement at 6.5 or above
//     fails, which keeps statements near a grade 7 reader.
//   - HarderThan stops an edit from making a statement harder to read, which is
//     how review loops compress text.
//
// Measured on 1,778 statements before shipping: median Dale-Chall score 5.4,
// 11% at 6.5 or above, 460 with a word outside everyday English.

// MaxStatementScore is the New Dale-Chall score a statement body must stay
// under. 6.0 to 6.9 is the grade 7-8 band; 6.5 keeps statements near grade 7.
// With the common-word additions the median statement scored 5.4 and 11%
// were at 6.5 or above when this shipped.
const MaxStatementScore = 6.5

// An edit may raise the score by at most maxScoreRise, once it lands at or
// above targetScore. Below the target a fix may add a condition freely.
const (
	maxScoreRise = 0.5
	targetScore  = 6.0
)

// familiarList holds everyday English words for UnfamiliarWords: every
// lowercase word with a Zipf frequency of 3.5 or more in wordfreq 3.1.1. To
// regenerate: wordfreq.top_n_list('en', 80000), keep [a-z]+ words with
// zipf_frequency >= 3.5.
//
//go:embed wordlists/familiar_en.txt
var familiarList string

// daleChallList is the Dale-Chall familiar-word list. It defines the scale of
// the Dale-Chall score and is used only for ReadingScore; it is too dated
// (it lacks "landlord" and "email") to judge single words.
//
//go:embed wordlists/dale_chall_en.txt
var daleChallList string

// commonList adds very common English words (Zipf 5.0 or more) to the
// Dale-Chall list for the score: the list dates from children's reading of
// the 1940s and misses "within", "problem" and "local".
//
//go:embed wordlists/common_en.txt
var commonList string

// renterWords are words our readers meet every day in their situation. Both
// word checks treat them as familiar; asking for a gloss on "landlord" in
// every statement would help no one.
var renterWords = strings.Fields(`landlord landlords lease leases leasing renter renters
	tenant tenants eviction evictions evict evicted evicting apartment apartments
	deposit deposits repair repairs rent rents rental rentals email emails online website
	websites internet phone text texts app court judge legal lawyer lawyers form forms
	hotline hotlines helpline helplines checklist weekday weekdays subtract roaches
	rodents thermostat railing railings remodel prepaid keyless outage stubs
	underline underlined reachable`)

// contraction matches the contractions everyday English is made of.
var contraction = regexp.MustCompile(`(?i)^[a-z]+('|’)(t|re|ll|ve|m|d)$`)

var familiar, daleChall = map[string]bool{}, map[string]bool{}

func init() {
	load := func(dst map[string]bool, list string) {
		for _, w := range strings.Split(list, "\n") {
			if w = strings.TrimSpace(w); w != "" && !strings.HasPrefix(w, "#") {
				dst[strings.ToLower(w)] = true
			}
		}
		for _, w := range renterWords {
			dst[w] = true
		}
	}
	load(familiar, familiarList)
	load(daleChall, daleChallList)
	load(daleChall, commonList)
}

var (
	wordTok      = regexp.MustCompile(`[A-Za-z][A-Za-z'’-]*`)
	sentenceTok  = regexp.MustCompile(`[.!?:;]+(\s|$)|\n+`)
	webAddress   = regexp.MustCompile(`(?i)https?://\S+|\S+\.(org|gov|com|net|us|info)(/\S*)?`)
	glossFollows = regexp.MustCompile(`^\s*\(`)
)

// inList reports whether w, or a regular inflection of it, is in list.
func inList(list map[string]bool, w string) bool {
	w = strings.TrimSuffix(strings.ReplaceAll(strings.ToLower(w), "’", "'"), "'s")
	w = strings.TrimSuffix(w, "'")
	if list[w] {
		return true
	}
	for _, s := range [][2]string{{"ies", "y"}, {"ied", "y"}, {"ier", "y"}, {"iest", "y"}, {"ily", "y"},
		{"es", ""}, {"s", ""}, {"ed", ""}, {"ed", "e"}, {"d", ""}, {"ing", ""}, {"ing", "e"},
		{"er", ""}, {"er", "e"}, {"est", ""}, {"ly", ""}, {"ness", ""}} {
		if !strings.HasSuffix(w, s[0]) || len(w) <= len(s[0])+1 {
			continue
		}
		stem := strings.TrimSuffix(w, s[0]) + s[1]
		if list[stem] {
			return true
		}
		// stopped, planning: a doubled final consonant.
		if n := len(stem); n > 2 && stem[n-1] == stem[n-2] && list[stem[:n-1]] {
			return true
		}
	}
	return false
}

// words walks the renter-facing words of a statement body and reports, for
// each, whether it is skipped: a capitalized name (a place, program, court),
// or a term glossed in parentheses right after it. Web addresses are dropped
// first.
func words(text string, fn func(w string, skipped bool)) {
	text = webAddress.ReplaceAllString(strings.ReplaceAll(text, "**", " "), " ")
	for _, loc := range wordTok.FindAllStringIndex(text, -1) {
		w := text[loc[0]:loc[1]]
		skipped := (w[0] >= 'A' && w[0] <= 'Z') || glossFollows.MatchString(text[loc[1]:])
		fn(w, skipped)
	}
}

func unfamiliarIn(list map[string]bool, w string) bool {
	if contraction.MatchString(w) {
		return false
	}
	for _, part := range strings.Split(w, "-") {
		if part != "" && !inList(list, part) {
			return true
		}
	}
	return false
}

// UnfamiliarWords returns the words in a statement body a renter may not
// know, each once, in order.
func UnfamiliarWords(text string) []string {
	var out []string
	seen := map[string]bool{}
	words(text, func(w string, skipped bool) {
		lw := strings.ToLower(w)
		if skipped || seen[lw] || !unfamiliarIn(familiar, w) {
			return
		}
		seen[lw] = true
		out = append(out, lw)
	})
	return out
}

// ReadingScore is the New Dale-Chall readability score of a statement body.
func ReadingScore(text string) float64 {
	total, hard := 0, 0
	words(text, func(w string, skipped bool) {
		total++
		if !skipped && unfamiliarIn(daleChall, w) {
			hard++
		}
	})
	if total == 0 {
		return 0
	}
	sents := 0
	for _, s := range sentenceTok.Split(webAddress.ReplaceAllString(text, " "), -1) {
		if wordTok.MatchString(s) {
			sents++
		}
	}
	if sents == 0 {
		sents = 1
	}
	pdw := 100 * float64(hard) / float64(total)
	score := 0.1579*pdw + 0.0496*float64(total)/float64(sents)
	if pdw > 5 {
		score += 3.6365
	}
	return score
}

// readabilityViolations is the statement-only readability rule.
func readabilityViolations(lang, text string) []string {
	if lang != "en" {
		return nil
	}
	var out []string
	if uw := UnfamiliarWords(text); len(uw) > 0 {
		out = append(out, fmt.Sprintf("words a renter may not know %q: use an everyday word, or keep an official term and explain it in parentheses right after it", uw))
	}
	if s := ReadingScore(text); s >= MaxStatementScore {
		out = append(out, fmt.Sprintf("reads too hard (Dale-Chall %.1f, must be under %.1f): use shorter sentences and everyday words, or split it into separate statements. Do not cut words by swapping in harder ones", s, MaxStatementScore))
	}
	return out
}

// HarderThan reports why a replacement body reads harder than the body it
// replaces, or "" when it does not. An edit may fix a fact, but it may not
// make the statement harder to read.
func HarderThan(lang, oldBody, newBody string) string {
	if lang != "en" || strings.TrimSpace(oldBody) == "" {
		return ""
	}
	if os, ns := ReadingScore(oldBody), ReadingScore(newBody); ns >= targetScore && ns > os+maxScoreRise {
		return fmt.Sprintf("the replacement reads harder than the current text (Dale-Chall %.1f, was %.1f): keep the words as plain as they were", ns, os)
	}
	had := map[string]bool{}
	for _, w := range UnfamiliarWords(oldBody) {
		had[w] = true
	}
	var added []string
	for _, w := range UnfamiliarWords(newBody) {
		if !had[w] {
			added = append(added, w)
		}
	}
	if len(added) > 0 {
		return fmt.Sprintf("the replacement adds words a renter may not know %q that the current text does not have", added)
	}
	return ""
}
