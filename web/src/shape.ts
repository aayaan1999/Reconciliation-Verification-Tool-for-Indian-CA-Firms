// A small runtime check that a JSON document has exactly the shape the screens expect:
// every key present with the right type, and no key the screens do not know. The tests
// run it on every sample file, so a change to the engine's --json output that the screens
// have not caught up with fails the build instead of showing a blank on screen.

type Spec =
  | 'string'
  | 'integer'
  | 'boolean'
  | 'integer|null'
  | { array: Spec }
  | { object: Record<string, Spec> }
  | { nullable: Spec }
  | { optional: Spec } // a key the engine leaves out when it has nothing to say
  | 'amounts'
  | 'texts'
  | 'value'

const valueSpecs: Record<string, Record<string, Spec>> = {
  money: { kind: 'string', paise: 'integer' },
  int: { kind: 'string', int: 'integer' },
  text: { kind: 'string', text: 'string' },
  date: { kind: 'string', date: 'string' },
  bool: { kind: 'string', bool: 'boolean' },
  rate: { kind: 'string', rate_bp: 'integer' },
}

function check(x: unknown, spec: Spec, at: string, out: string[]): void {
  if (spec === 'string') {
    if (typeof x !== 'string') out.push(`${at}: want text`)
  } else if (spec === 'integer') {
    if (!Number.isSafeInteger(x)) out.push(`${at}: want a whole number`)
  } else if (spec === 'integer|null') {
    if (x !== null && !Number.isSafeInteger(x)) out.push(`${at}: want a whole number or null`)
  } else if (spec === 'boolean') {
    if (typeof x !== 'boolean') out.push(`${at}: want true or false`)
  } else if (spec === 'value') {
    const kind = (x as { kind?: unknown } | null)?.kind
    const vs = typeof kind === 'string' ? valueSpecs[kind] : undefined
    if (!vs) out.push(`${at}: unknown value kind ${String(kind)}`)
    else check(x, { object: vs }, at, out)
  } else if (spec === 'amounts') {
    if (typeof x !== 'object' || x === null || Array.isArray(x)) out.push(`${at}: want an object of amounts`)
    else for (const [k, v] of Object.entries(x)) if (!Number.isSafeInteger(v)) out.push(`${at}.${k}: want a whole number`)
  } else if (spec === 'texts') {
    if (typeof x !== 'object' || x === null || Array.isArray(x)) out.push(`${at}: want an object of texts`)
    else for (const [k, v] of Object.entries(x)) if (typeof v !== 'string') out.push(`${at}.${k}: want text`)
  } else if ('nullable' in spec) {
    if (x !== null) check(x, spec.nullable, at, out)
  } else if ('optional' in spec) {
    check(x, spec.optional, at, out) // absence is handled by the object that holds the key
  } else if ('array' in spec) {
    if (!Array.isArray(x)) out.push(`${at}: want a list`)
    else x.forEach((item, i) => check(item, spec.array, `${at}[${i}]`, out))
  } else {
    if (typeof x !== 'object' || x === null || Array.isArray(x)) {
      out.push(`${at}: want an object`)
      return
    }
    const obj = x as Record<string, unknown>
    for (const key of Object.keys(spec.object)) {
      const want = spec.object[key]
      if (typeof want === 'object' && 'optional' in want) {
        if (key in obj) check(obj[key], want.optional, `${at}.${key}`, out)
      } else if (!(key in obj)) out.push(`${at}.${key}: missing`)
      else check(obj[key], want, `${at}.${key}`, out)
    }
    for (const key of Object.keys(obj)) {
      if (!(key in spec.object)) out.push(`${at}.${key}: not known to the screens`)
    }
  }
}

const subject: Spec = { object: { period: 'string', scope: 'string', match_key: 'string' } }
const issue: Spec = { object: { code: 'string', message: 'string', path: 'string' } }
const locator: Spec = {
  object: { document_version_id: 'integer', file_name: 'string', kind: 'string', path: 'string' },
}
const storedLocator: Spec = { object: { file_name: 'string', kind: 'string', path: 'string' } }

export const runReportSpec: Spec = {
  object: {
    tax_year: 'string',
    includes_testing: 'boolean',
    counts: { object: { checks: 'integer', pass: 'integer', fail: 'integer', cannot_check: 'integer' } },
    files: {
      array: {
        object: {
          file: 'string', file_type: 'string', document_type: 'string', profile_id: 'string', profile_version: 'string',
          source_system: 'string', tax_year: 'string', client_match: 'string', pan_last4: 'string',
          fact_count: 'integer', record_count: 'integer',
          unread: { array: { object: { path: 'string', kind: 'string', row: 'string', reason: 'string' } } },
          warnings: { array: issue },
          errors: { array: issue },
        },
      },
    },
    findings: {
      array: {
        object: {
          review_document: 'string',
          heading: 'string',
          findings: {
            array: {
              object: {
                finding_id: 'string', value_fingerprint: 'string',
                rule_id: 'string', rule_version: 'string', title: 'string', category: 'string',
                code: 'string', code_text: 'string', subject,
                values: { array: { object: { input: 'string', label: 'string', value: 'value', locator } } },
                difference_paise: 'integer', show_difference: 'boolean',
                law_refs: { array: { object: { act: 'string', section: 'string', reference: 'string' } } },
              },
            },
          },
        },
      },
    },
    cannot_check: {
      array: {
        object: {
          rule_id: 'string', rule_version: 'string', title: 'string', category: 'string', subject,
          missing: {
            array: { object: { fact: 'string', label: 'string', document_type: 'string', document: 'string', rows: 'boolean' } },
          },
        },
      },
    },
    passed: {
      array: {
        object: {
          rule_id: 'string', rule_version: 'string', title: 'string', category: 'string',
          review_document: 'string', code: 'string', subject,
          // What the check compared; a comparison kept before these were stored has none of them.
          code_text: { optional: 'string' },
          values: { optional: { array: { object: { input: 'string', label: 'string', value: 'value', locator } } } },
          difference_paise: { optional: 'integer' }, show_difference: { optional: 'boolean' },
          tolerance: { optional: { object: { kind: 'string', value: 'integer' } } },
        },
      },
    },
    comparison: {
      nullable: {
        object: {
          id: 'string', name: 'string', description: 'string', totals: 'boolean',
          documents: { array: { object: { document_type: 'string', name: 'string', present: 'boolean' } } },
        },
      },
    },
    totals: {
      array: { object: { document_type: 'string', document: 'string', rows: 'integer', amounts: 'amounts' } },
    },
    not_selected: { array: { object: { rule_id: 'string', title: 'string' } } },
  },
}

export const ruleListSpec: Spec = {
  object: {
    tax_years: { array: 'string' },
    rules: {
      array: {
        object: {
          id: 'string', version: 'string', revision: 'integer', source: 'string', status: 'string',
          runnable: 'boolean', type: 'string', domain: 'string', category: 'string', also_categories: { optional: { array: 'string' } },
          title: 'string', tax_years: { array: 'string' }, valid_from: 'string', valid_to: 'string', file_path: 'string',
          approvable: 'boolean', approval_note: 'string', reads_office_documents: 'boolean',
        },
      },
    },
  },
}

export const factListSpec: Spec = {
  object: {
    engagements: {
      array: {
        object: {
          tax_year: 'string',
          documents: {
            array: {
              object: {
                document_type: 'string', period: 'string', scope: 'string', part: 'string',
                file_name: 'string', file_sha256: 'string', draft_no: 'integer',
                profile_id: 'string', profile_version: 'string',
                facts: {
                  array: { object: { name: 'string', period: 'string', scope: 'string', value: 'value', locator: storedLocator } },
                },
                records: {
                  array: {
                    object: {
                      record_type: 'string', match_key: 'string', period: 'string', scope: 'string',
                      fields: 'texts', amounts: 'amounts', locator: storedLocator,
                    },
                  },
                },
              },
            },
          },
        },
      },
    },
  },
}

export const clientsSpec: Spec = {
  object: {
    note: 'string',
    tax_year: 'string',
    clients: {
      array: { object: { id: 'string', name: 'string', pan_last4: 'string', runs: { array: 'string' }, last_run_on: 'string' } },
    },
  },
}

export const worklistSpec: Spec = {
  object: {
    tax_year: 'string',
    items: {
      array: {
        object: {
          client_id: 'integer', client_name: 'string', key: 'string', kind: 'string', kind_text: 'string',
          ledger: 'string', group: 'string', text: 'string', prior_total: 'integer', current_total: 'integer',
          prior_months: 'integer', current_months: 'integer', missing_months: { array: 'string' }, times: 'string',
          prior_rows: { array: 'string' }, current_rows: { array: 'string' },
          classification: { nullable: { object: { text: 'string', by: 'string', at: 'string' } } },
        },
      },
    },
  },
}

const counts: Spec = { object: { checks: 'integer', pass: 'integer', fail: 'integer', cannot_check: 'integer' } }

export const serverClientSpec: Spec = {
  object: { id: 'integer', name: 'string', pan_last4: 'string', last_run_on: 'string', documents: 'integer', counts: { nullable: counts } },
}

export const serverClientListSpec: Spec = { object: { tax_year: 'string', clients: { array: serverClientSpec } } }

export const comparisonListSpec: Spec = {
  object: {
    comparisons: {
      array: {
        object: {
          id: 'string', name: 'string', description: 'string', documents: { array: 'string' }, rules: { array: 'string' },
          totals: 'boolean', worklist: 'boolean', document_names: { array: { object: { document_type: 'string', name: 'string' } } },
        },
      },
    },
  },
}

// shapeProblems returns every place where x differs from spec; none means it fits.
export function shapeProblems(x: unknown, spec: Spec): string[] {
  const out: string[] = []
  check(x, spec, '$', out)
  return out
}
