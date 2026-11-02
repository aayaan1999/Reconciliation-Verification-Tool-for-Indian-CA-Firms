import { pageSizes, type Pager } from '../pager.ts'

// PageBar sits below a list: how many to show at a time, which ones are shown, and the way
// to the previous and next pages. A list that fits on the smallest page has none.
export function PageBar<T>({ p, noun }: { p: Pager<T>; noun: string }) {
  if (p.total <= pageSizes[0]) return null
  return (
    <div className="pager pager-below card-premium-enhanced" role="group" aria-label={`Pages of ${noun}`}>
      <label className="form-group-premium" style={{ marginBottom: 0 }}>
        Show
        <select aria-label={`${noun[0].toUpperCase()}${noun.slice(1)} per page`} value={p.size} onChange={(e) => p.setSize(Number(e.target.value))}>
          {pageSizes.map((n) => (
            <option key={n} value={n}>
              {n}
            </option>
          ))}
        </select>
        per page
      </label>
      <span className="pager-range" aria-live="polite">
        {p.start + 1}–{Math.min(p.start + p.size, p.total)} of {p.total}
      </span>
      <span className="pager-steps">
        <button type="button" onClick={() => p.setPage(p.page - 1)} disabled={p.page === 0}>
          Previous
        </button>
        <span>
          Page {p.page + 1} of {p.pages}
        </span>
        <button type="button" onClick={() => p.setPage(p.page + 1)} disabled={p.page >= p.pages - 1}>
          Next
        </button>
      </span>
    </div>
  )
}
