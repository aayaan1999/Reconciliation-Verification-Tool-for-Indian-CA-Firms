// The phrases that may never appear in text shown with a finding (CLAUDE.md invariant 1).
// Kept equal to rules.BannedPhrases in internal/rules/wording.go; a Go test compares them.
export const bannedPhrases = ['should', 'incorrect', 'must be', 'wrong', 'correct value', 'ought', 'required to']

// bannedIn returns the banned phrases in text, matched as whole words, ignoring case.
export function bannedIn(text: string): string[] {
  const found: string[] = []
  for (const p of bannedPhrases) {
    const re = new RegExp(`\\b${p.replace(/ /g, '\\s+')}\\b`, 'i')
    if (re.test(text)) found.push(p)
  }
  return found
}
