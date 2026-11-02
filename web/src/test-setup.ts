import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterEach } from 'vitest'

// Vitest globals are off, so Testing Library cannot unmount between tests by itself.
afterEach(cleanup)

// findBy… and waitFor wait up to 5 s (default 1 s): in a full, parallel run on an office PC a
// popup that opens after a server reply can take longer than 1 s to appear.
configure({ asyncUtilTimeout: 5000 })
