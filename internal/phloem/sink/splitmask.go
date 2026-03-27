package sink

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maskReplacement pairs a placeholder (no '.' or sentence punctuation) with the original span.
// Rules mirror python/nlp_worker/mask.py.
type maskReplacement struct {
	Placeholder string
	Original    string
}

type splitMaskSpan struct {
	start, end int
	text       string
}

var (
	reMaskMdLink  = regexp.MustCompile(`!?\[[^\]]*\]\([^)]*\)`)
	reMaskURL     = regexp.MustCompile(`https?://[^\s\]>]+`)
	reMaskSemver  = regexp.MustCompile(`\bv\d+(?:\.\d+)+`)
	reMaskSection    = regexp.MustCompile(`\d+\.\d+\.\d+(?:~\d+\.\d+\.\d+)?`)
	reMaskFileExt    = regexp.MustCompile(`(?i)(?:[\w.-]+/)*[\w.-]+\.(?:md|go|py|txt|yml|yaml|json|proto)\b`)
	reMaskListMarker = regexp.MustCompile(`(?m)^\s{0,3}\d+\.`)
)

func validSectionBounds(text string, start, end int) bool {
	if start > 0 {
		prev, _ := utf8.DecodeLastRuneInString(text[:start])
		if unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '.' || prev == '_' {
			return false
		}
	}
	if end < len(text) {
		next, _ := utf8.DecodeRuneInString(text[end:])
		if unicode.IsDigit(next) || next == '.' {
			return false
		}
	}
	return true
}

func validFileBounds(text string, start int) bool {
	if start == 0 {
		return true
	}
	prev, _ := utf8.DecodeLastRuneInString(text[:start])
	if prev == '/' {
		return true
	}
	if unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '.' || prev == '_' || prev == '-' {
		return false
	}
	return true
}

func validListMarkerBounds(text string, start, end int) bool {
	if end < len(text) {
		next, _ := utf8.DecodeRuneInString(text[end:])
		if !unicode.IsSpace(next) {
			return false
		}
	}
	return true
}

func maskPlaceholder(i int) string {
	// Match python/nlp_worker/mask.py: "__GOPEDIA_N__"
	return fmt.Sprintf("__GOPEDIA_%d__", i)
}

// maskForSentenceSplit replaces link-like and dotted spans so '.' inside them does not end sentences.
func maskForSentenceSplit(text string) (string, []maskReplacement) {
	var cand []splitMaskSpan
	add := func(s, e int) {
		if s < 0 || e > len(text) || s >= e {
			return
		}
		cand = append(cand, splitMaskSpan{s, e, text[s:e]})
	}
	for _, loc := range reMaskMdLink.FindAllStringIndex(text, -1) {
		add(loc[0], loc[1])
	}
	for _, loc := range reMaskURL.FindAllStringIndex(text, -1) {
		add(loc[0], loc[1])
	}
	for _, loc := range reMaskSemver.FindAllStringIndex(text, -1) {
		add(loc[0], loc[1])
	}
	for _, loc := range reMaskSection.FindAllStringIndex(text, -1) {
		if !validSectionBounds(text, loc[0], loc[1]) {
			continue
		}
		add(loc[0], loc[1])
	}
	for _, loc := range reMaskFileExt.FindAllStringIndex(text, -1) {
		if !validFileBounds(text, loc[0]) {
			continue
		}
		add(loc[0], loc[1])
	}
	for _, loc := range reMaskListMarker.FindAllStringIndex(text, -1) {
		if !validListMarkerBounds(text, loc[0], loc[1]) {
			continue
		}
		add(loc[0], loc[1])
	}
	if len(cand) == 0 {
		return text, nil
	}
	sort.Slice(cand, func(i, j int) bool {
		return (cand[i].end-cand[i].start) > (cand[j].end-cand[j].start)
	})
	var chosen []splitMaskSpan
outer:
	for _, sp := range cand {
		for _, ch := range chosen {
			if !(sp.end <= ch.start || sp.start >= ch.end) {
				continue outer
			}
		}
		chosen = append(chosen, sp)
	}
	sort.Slice(chosen, func(i, j int) bool { return chosen[i].start < chosen[j].start })

	var b strings.Builder
	var repls []maskReplacement
	last := 0
	for i, sp := range chosen {
		b.WriteString(text[last:sp.start])
		ph := maskPlaceholder(i)
		repls = append(repls, maskReplacement{Placeholder: ph, Original: sp.text})
		b.WriteString(ph)
		last = sp.end
	}
	b.WriteString(text[last:])
	return b.String(), repls
}

func unmaskSplitText(s string, repls []maskReplacement) string {
	for _, r := range repls {
		s = strings.ReplaceAll(s, r.Placeholder, r.Original)
	}
	return s
}
