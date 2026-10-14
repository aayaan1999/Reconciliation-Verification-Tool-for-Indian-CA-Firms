package rules

import (
	"regexp"
	"strings"
)

// BannedPhrases may never appear in text that reaches a finding: rule titles, input labels
// and (SPEC-009) finding templates. Findings state what was compared, never what ought to
// be (CLAUDE.md invariant 1, technical spec §B4.6).
var BannedPhrases = []string{"should", "incorrect", "must be", "wrong", "correct value", "ought", "required to"}

var bannedPattern = func() *regexp.Regexp {
	alts := make([]string, len(BannedPhrases))
	for i, p := range BannedPhrases {
		alts[i] = strings.ReplaceAll(regexp.QuoteMeta(p), ` `, `\s+`)
	}
	return regexp.MustCompile(`(?i)\b(?:` + strings.Join(alts, "|") + `)\b`)
}()

// BannedIn returns the banned phrases in text, in order of appearance, matched as whole
// words and ignoring case ("Should" and "MUST  BE" match; "shoulder" does not).
func BannedIn(text string) []string {
	var found []string
	for _, m := range bannedPattern.FindAllString(text, -1) {
		found = append(found, strings.ToLower(strings.Join(strings.Fields(m), " ")))
	}
	return found
}
