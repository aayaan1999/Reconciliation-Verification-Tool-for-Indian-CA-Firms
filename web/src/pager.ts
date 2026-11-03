import { useState } from 'react'

// The page sizes offered; 10 is the default. A screen keeps the size chosen in this browser.
export const pageSizes = [5, 10, 15, 30]
const defaultSize = 10
const sizeKey = (name: string) => `recon.pageSize.${name}`

function storedSize(name: string): number {
  try {
    const n = Number(localStorage.getItem(sizeKey(name)))
    return pageSizes.includes(n) ? n : defaultSize
  } catch {
    return defaultSize
  }
}

export interface Pager<T> {
  rows: T[] // the items of the page shown
  page: number // from 0
  pages: number
  size: number
  start: number // index of the page's first item
  total: number
  setPage: (page: number) => void
  setSize: (size: number) => void
}

// usePager shows a list a page at a time. reset names what the list is filtered by: when it
// changes, the first page is shown again.
export function usePager<T>(items: T[], name: string, reset = ''): Pager<T> {
  const [size, setSizeState] = useState(() => storedSize(name))
  const [page, setPageState] = useState(0)
  const [lastReset, setLastReset] = useState(reset)
  let current = page
  if (reset !== lastReset) {
    setLastReset(reset)
    setPageState(0)
    current = 0
  }
  const pages = Math.max(1, Math.ceil(items.length / size))
  current = Math.min(current, pages - 1)
  const start = current * size
  const setPage = (p: number) => setPageState(Math.max(0, Math.min(p, pages - 1)))
  const setSize = (n: number) => {
    setSizeState(n)
    setPageState(0)
    try {
      localStorage.setItem(sizeKey(name), String(n))
    } catch {
      // Not kept in this browser: 10 a page next time.
    }
  }
  return { rows: items.slice(start, start + size), page: current, pages, size, start, total: items.length, setPage, setSize }
}
