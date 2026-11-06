import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'

// Vitest globals are off, so Testing Library cannot unmount between tests by itself.
afterEach(cleanup)

// The screens open on the current tax year, so the date is fixed: 4 November 2026 is in tax
// year 2026-27. Only Date is faked; timers and promises run as usual.
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 10, 4, 12, 0, 0))
})
afterEach(() => {
  vi.useRealTimers()
})

// findBy… and waitFor wait up to 5 s (default 1 s): in a full, parallel run on an office PC a
// popup that opens after a server reply can take longer than 1 s to appear.
configure({ asyncUtilTimeout: 5000 })
