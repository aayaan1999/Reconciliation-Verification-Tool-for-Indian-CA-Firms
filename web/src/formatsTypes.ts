import type { LayoutMapping } from './types.ts'

// What the Formats endpoints return (SPEC-016). A format is a profile and a parameter is a fact or
// record type of the vocabulary, so the screen speaks the engine's own words.

export interface FormatVersionInfo {
  revision: number
  version: string // "v1"
  status: string // draft | testing | active | disabled
  by: string
  at: string
  note: string
  published: boolean // false for a draft
}

export interface FormatInfo {
  id: string
  name: string
  file_type: string // json | xlsx | csv
  document: string
  source: string // builtin | office
  version: string
  revision: number
  status: string
  disabled_reason: string
  published_by: string
  reads_facts: string[]
  reads_records: string[]
  versions: FormatVersionInfo[]
  samples: KeptSample[]
  last_used: string // "" when no file was read with it
  needed: number // figures of its document that the rules read
  needed_read: number // of those, the ones this format reads
}

export interface RuleUse {
  id: string
  title: string
  status: string
}

export interface ParameterInfo {
  key: string
  name: string
  kind: string // money | int | text | date | bool | rate | record
  document: string
  description: string
  aliases: string[]
  source: string
  status: string // active | disabled
  disabled_reason: string
  deprecated: boolean
  version: string
  used_by_formats: string[] // format IDs
  used_by_rules: RuleUse[]
}

export interface UncoveredRule extends RuleUse {
  missing: string[]
}

export interface FormatsList {
  formats: FormatInfo[]
  parameters: ParameterInfo[]
  unsupplied: string[] // keys of parameters no active format reads
  uncovered: UncoveredRule[]
  unrecognised: UnrecognisedInfo[]
}

export interface NewParameter {
  key: string
  name: string
  kind: string
  document: string
  description: string
  aliases: string[]
}

export interface DryRule {
  id: string
  title: string
  missing?: string[] // the figures of the layout's document that the layout does not read
}

// The dry run of a layout: which rules that read its document can run, and which cannot check.
export interface DryRun {
  can_run: DryRule[]
  cannot_check: DryRule[]
}

export interface MappingView {
  revision: number
  version: string
  mapping: LayoutMapping
}

// What format-get returns: the mapping of an office format, for the mapping screen to edit.
export interface FormatMapping {
  id: string
  name: string
  editable: boolean
  reason: string // why it cannot be edited
  in_use: MappingView | null
  draft: MappingView | null // the latest version not yet published
}

// A file no format recognised, waiting for one (SPEC-016). Only its name, hash and size are kept.
export interface UnrecognisedInfo {
  id: number
  file_name: string
  file_type: string
  sha256: string
  size_bytes: number
  tax_year: string
  noted_at: string
}

// A sample file kept for a format, so that publishing a change can read it again.
export interface KeptSample {
  id: number
  file_name: string
  size_bytes: number
  kept_by: string
  kept_at: string
  has_content: boolean
}

// What an import of a bundle would add, or added.
export interface ImportPlan {
  format: string
  format_name: string
  parameters_added: string[]
  parameters_unchanged: string[]
  applied: boolean
  profile_folder: string
}
