import type { Value } from './types.ts'

// formatPaise writes an amount of paise in rupees with Indian digit grouping, as the engine
// report does: 11245050 → "1,12,450.50". It uses whole-number arithmetic (BigInt) only,
// so no amount is ever rounded on screen.
export function formatPaise(paise: number, showPaise = true): string {
  if (!Number.isSafeInteger(paise)) {
    throw new Error(`amount ${paise} is not a whole number of paise`)
  }
  const negative = paise < 0
  const mag = negative ? -BigInt(paise) : BigInt(paise)
  const rupees = (mag / 100n).toString()
  const rest = (mag % 100n).toString().padStart(2, '0')
  let grouped = rupees
  if (rupees.length > 3) {
    const head = rupees.slice(0, -3)
    const tail = rupees.slice(-3)
    grouped = head.replace(/\B(?=(\d{2})+$)/g, ',') + ',' + tail
  }
  return (negative ? '-' : '') + grouped + (showPaise || rest !== '00' ? '.' + rest : '')
}

// formatValue writes any engine value for display. Money keeps its paise.
export function formatValue(v: Value): string {
  switch (v.kind) {
    case 'money':
      return formatPaise(v.paise)
    case 'int':
      return String(v.int)
    case 'text':
      return v.text.trim() === '' ? '(blank)' : v.text
    case 'date':
      return v.date
    case 'bool':
      return v.bool ? 'Yes' : 'No'
    case 'rate':
      return formatPaise(v.rate_bp) + '%' // basis points: 1250 → "12.50%"
  }
}

// initials are the first letters of the first two words of a name, for an avatar
// ("Sample Exports LLP" → "SE"). Words that do not start with a letter are skipped.
export function initials(name: string): string {
  return name.split(/\s+/).filter((w) => /^[A-Za-z]/.test(w)).slice(0, 2).map((w) => w[0].toUpperCase()).join('')
}

// locatorText is the short "file place" text the engine report shows as a source.
export function locatorText(l: { file_name: string; path: string }): string {
  return l.path ? `${l.file_name} ${l.path}` : l.file_name
}
