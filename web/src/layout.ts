// Helpers of the mapping screen (SPEC-004 part 8.4): the fields a row offers and first
// guesses of which field is which column.
import type { InspectedField } from './types.ts'

// The words shown for each column of a row.
export const columnText: Record<string, string> = {
  invoice_no: 'Invoice number', invoice_date: 'Invoice date', counterparty_gstin: 'Customer GSTIN',
  supplier_gstin: 'Supplier GSTIN', place_of_supply: 'Place of supply', rate: 'GST rate', itc_available: 'ITC available',
  taxable_value: 'Taxable value', igst: 'IGST', cgst: 'CGST', sgst: 'SGST/UTGST', cess: 'Cess', invoice_value: 'Invoice value',
}

// Field names that usually hold each column, most telling first. Only a first guess: the
// user sees every choice before anything is saved.
const aliases: Record<string, string[]> = {
  invoice_no: ['invoiceno', 'invoicenumber', 'inum', 'invno', 'billno', 'voucherno', 'docno', 'documentno'],
  invoice_date: ['invoicedate', 'idt', 'invdate', 'billdate', 'voucherdate', 'docdate', 'date'],
  counterparty_gstin: ['counterpartygstin', 'partygstin', 'buyergstin', 'customergstin', 'recipientgstin', 'ctin'],
  supplier_gstin: ['suppliergstin', 'vendorgstin', 'partygstin', 'ctin'],
  place_of_supply: ['placeofsupply', 'pos'],
  rate: ['gstrate', 'taxrate', 'rt'],
  itc_available: ['itcavailable', 'itcavl', 'itc'],
  taxable_value: ['taxablevalue', 'taxable', 'txval', 'taxableamount', 'assessablevalue'],
  igst: ['igst', 'iamt', 'igstamount'],
  cgst: ['cgst', 'camt', 'cgstamount'],
  sgst: ['sgst', 'sgstutgst', 'samt', 'utgst', 'sgstamount'],
  cess: ['cess', 'csamt', 'cessamount'],
  invoice_value: ['invoicevalue', 'val', 'invoicetotal', 'grandtotal', 'totalvalue', 'total'],
}

export const norm = (s: string) => s.toLowerCase().replace(/[^a-z0-9]/g, '')
export const gstinPattern = /^\d{2}[A-Z]{5}\d{4}[A-Z][1-9A-Z]Z[0-9A-Z]$/

// lastKey is the name of the last member in a path: ".Amounts.Taxable" → "Taxable".
export function lastKey(path: string): string {
  const q = path.match(/\['((?:[^'\\]|\\.)*)'\]$/)
  if (q) return q[1]
  return path.slice(path.lastIndexOf('.') + 1)
}

// parentOf is the object around a list's items: "$.b2b[*].inv[*]" → "$.b2b[*]".
function parentOf(rows: string): string {
  const list = rows.replace(/\[\*\]$/, '')
  const q = list.lastIndexOf("['")
  const d = list.lastIndexOf('.')
  return list.slice(0, Math.max(q, d))
}

export interface FieldOption {
  path: string // as written in the mapping: ".inv_no", "^.ctin" or "$.fp"
  example: string
}

// rowOptions lists the fields of one row (written from the row) and, for a list inside
// another list, the fields of the object around it (written with "^").
export function rowOptions(fields: InspectedField[], rows: string): FieldOption[] {
  if (!rows) return []
  const out: FieldOption[] = []
  const parent = parentOf(rows)
  for (const f of fields) {
    if (f.path.startsWith(rows) && (f.path[rows.length] === '.' || f.path[rows.length] === '[')) {
      out.push({ path: f.path.slice(rows.length), example: f.example })
    }
  }
  if (parent.includes('[*]')) {
    for (const f of fields) {
      const rest = f.path.slice(parent.length)
      if (f.path.startsWith(parent) && !f.path.startsWith(rows) && (rest[0] === '.' || rest.startsWith("['")) && !rest.includes('[*]')) {
        out.push({ path: '^' + rest, example: f.example })
      }
    }
  }
  return out
}

// guessColumn picks the field whose name is a usual name for the column.
export function guessColumn(column: string, options: FieldOption[]): string {
  for (const a of aliases[column] ?? [norm(column)]) {
    const hit = options.find((o) => norm(lastKey(o.path)) === a)
    if (hit) return hit.path
  }
  return ''
}

// columnsOf names the columns each field is mapped to, so the file's values can be labelled
// with them: {invoice_no: ".bill_no"} → {".bill_no": ["invoice_no"]}.
export function columnsOf(columns: Record<string, string>): Record<string, string[]> {
  const out: Record<string, string[]> = {}
  for (const [column, path] of Object.entries(columns)) {
    if (path) (out[path] ??= []).push(column)
  }
  return out
}

// periodFormat says how a period example is written.
export function periodFormat(example: string): string {
  if (/^\d{6}$/.test(example)) return Number(example.slice(0, 2)) <= 12 ? 'mmyyyy' : 'yyyymm'
  return ''
}

// singleFields are the fields that hold one value for the whole file, for the period, the
// client's GSTIN and an identifying value. In JSON they are outside every list. In an Excel
// workbook every value is in a sheet's table, so they are the columns of a sheet
// ("$['Details'][*]['Return Period']"), read from its first row (SPEC-008 part 2).
export function singleFields(fields: InspectedField[], excel: boolean): InspectedField[] {
  return fields.filter((f) => (excel ? f.path.split('[*]').length === 2 : !f.path.includes('[*]')))
}
