// The shapes of the Document Master (SPEC-022): what /api/v1/document-types returns and takes.
// The engine decides what a valid document is; these only describe the answers.

export type ColumnType = 'text' | 'date' | 'amount'
export type DocStatus = 'draft' | 'active' | 'inactive'
export type DocOrigin = 'builtin' | 'office'
export type FileKind = 'json' | 'xlsx' | 'csv'

// The file formats a document can accept, in the order the engine stores them.
export const fileKinds: FileKind[] = ['json', 'xlsx', 'csv']
export const fileKindNames: Record<FileKind, string> = { json: 'JSON', xlsx: 'Excel (XLSX)', csv: 'CSV' }

// The fixed list a document's category comes from (internal/vocab/officecheck.go).
export const docCategories = ['Books', 'GST', 'Income tax', 'TDS', 'Payroll', 'Bank', 'Other']

export interface DocColumn {
  name: string
  label: string
  type: ColumnType
  key: boolean
  required: boolean
}

export interface DocRecordType {
  name: string
  description: string
  copied_from: string
  locked: boolean // something reads it: only labels, the description and new optional columns may change
  columns: DocColumn[]
}

export interface DocUsage {
  formats: string[]
  rules: string[]
  active_rules: string[]
  records: number
}

export interface DocRow {
  code: string
  origin: DocOrigin
  name: string
  category: string
  description: string
  applicable_for: string[]
  status: DocStatus
  record_type: string
  updated_by: string
  updated_at: string
  referenced_by: { formats: number; rules: number }
}

export interface DocHistoryEntry {
  at: string
  by: string
  action: string
  details: string
}

export interface DocDetail extends DocRow {
  structure: DocRecordType | null
  usage: DocUsage
  history: DocHistoryEntry[]
}

export interface DocProblem {
  field: string
  code: string
  message: string
}

// What the screen sends to add or edit a document. record_type is left out for a Draft that has no
// structure yet; with copy_from and no columns the engine copies the columns of that record type.
export interface DocInput {
  code: string
  name: string
  category: string
  description: string
  applicable_for: string[]
  status?: 'active' | 'inactive'
  record_type?: { name: string; description: string; copy_from?: string; columns?: DocColumn[] }
}

export interface DocValidation {
  valid: boolean
  problems: DocProblem[]
  record_type: { name: string; description: string; copied_from: string; columns: DocColumn[] } | null
}

export interface DocSaved {
  saved: boolean
  problems: DocProblem[] // for a Draft: what is missing
  document: DocDetail
}

export interface DocDeleted {
  deleted: boolean
}

// formatsText writes the formats of a document for a table cell: JSON, XLSX, CSV.
export const formatsText = (formats: string[]) => fileKinds.filter((k) => formats.includes(k)).map((k) => k.toUpperCase()).join(', ') || 'None'
