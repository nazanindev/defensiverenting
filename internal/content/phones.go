package content

import (
	"html/template"
	"regexp"
	"strconv"
	"strings"
)

// Phone numbers in a statement become tap-to-call links (ADR-029 D6), so a
// renter on a phone can call in one tap and the tap can be counted against
// the source the statement cites.
var (
	// A US number: optional 1, then 3-3-4 digits with (), dash, dot or space.
	phoneRE = regexp.MustCompile(`(?:\+?1[-.\s])?(?:\(\d{3}\)\s?|\d{3}[-.\s])\d{3}[-.\s]\d{4}`)
	// The three-digit lines, only where the sentence says to call one, so a
	// section number like "§ 211" is never a link.
	shortRE = regexp.MustCompile(`(?i)\b(?:call|dial|text)\s+(2-?1-?1|3-?1-?1)\b`)
)

// LinkPhones wraps each phone number in rendered statement HTML in a tel:
// link. Text inside tags and inside existing links is left alone. sourceID,
// when not 0, rides on the link as data-out so the page script can report the
// tap against that source.
func LinkPhones(h template.HTML, sourceID int64) template.HTML {
	s := string(h)
	var b strings.Builder
	inLink := 0
	for len(s) > 0 {
		lt := strings.IndexByte(s, '<')
		if lt < 0 {
			lt = len(s)
		}
		if inLink == 0 {
			b.WriteString(linkText(s[:lt], sourceID))
		} else {
			b.WriteString(s[:lt])
		}
		s = s[lt:]
		if s == "" {
			break
		}
		gt := strings.IndexByte(s, '>')
		if gt < 0 {
			b.WriteString(s)
			break
		}
		tag := strings.ToLower(s[:gt+1])
		switch {
		case strings.HasPrefix(tag, "<a ") || tag == "<a>":
			inLink++
		case strings.HasPrefix(tag, "</a"):
			if inLink > 0 {
				inLink--
			}
		}
		b.WriteString(s[:gt+1])
		s = s[gt+1:]
	}
	//nolint:gosec // Rewrites already-rendered statement HTML; only adds anchors around digit runs.
	return template.HTML(b.String())
}

func linkText(text string, sourceID int64) string {
	out := phoneRE.ReplaceAllStringFunc(text, func(m string) string {
		digits := onlyDigits(m)
		if len(digits) == 10 {
			digits = "1" + digits
		}
		return telLink("+"+digits, m, sourceID)
	})
	return shortRE.ReplaceAllStringFunc(out, func(m string) string {
		sub := shortRE.FindStringSubmatchIndex(m)
		num := m[sub[2]:sub[3]]
		return m[:sub[2]] + telLink(onlyDigits(num), num, sourceID) + m[sub[3]:]
	})
}

func telLink(dial, shown string, sourceID int64) string {
	attr := ""
	if sourceID != 0 {
		attr = ` data-out="` + strconv.FormatInt(sourceID, 10) + `"`
	}
	return `<a href="tel:` + dial + `"` + attr + `>` + shown + `</a>`
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
