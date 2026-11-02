// Decisions on findings (SPEC-006 part 3): the words and a finding's state. The server records
// the signed-in user as the decider (SPEC-012), so the screens no longer ask for a name.
import type { DecisionKind, Finding, FindingDecision } from './types.ts'

export type DecideFn = (findingId: string, decision: DecisionKind, note: string) => Promise<void>

export const decisionText: Record<DecisionKind, string> = { fix: 'Fix', not_an_issue: 'Not an issue', ask_client: 'Ask client' }

// decisionMeaning says what each decision means, on its card (Recon Screen Document v2 §7.4).
export const decisionMeaning: Record<DecisionKind, string> = {
  fix: 'The document will be corrected',
  not_an_issue: 'The difference is explained',
  ask_client: 'Information is needed from the client',
}

// decisionKeys are the single keys that apply each decision to the finding shown.
export const decisionKeys: Record<DecisionKind, string> = { fix: 'f', not_an_issue: 'x', ask_client: 'a' }

// stateText names a finding's state for the filter and the tag: open, or the decision.
export const stateText: Record<string, string> = { open: 'Open', ...decisionText }

// decisionState is a finding's state: its current decision, or open (no decision yet, or the
// figures changed after the decision).
export function decisionState(decisions: Record<string, FindingDecision>, f: Finding): string {
  return decisions[f.finding_id]?.state ?? 'open'
}
