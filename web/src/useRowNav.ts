import { useCallback, useState, type KeyboardEvent } from 'react'

// useRowNav makes a list usable from the keyboard: ArrowDown and ArrowUp (or j and k) move
// the selection, Home and End jump to the ends, and Enter opens the selected row. The
// list element gets onKeyDown and tabIndex; each row reads `selected`.
export function useRowNav(count: number, onOpen: (index: number) => void) {
  const [selected, setSelected] = useState(0)
  const current = count === 0 ? -1 : Math.min(selected, count - 1)
  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      // Ctrl K opens the search; with a modifier held no key here is a move.
      if (count === 0 || e.ctrlKey || e.metaKey || e.altKey) return
      let next = current
      switch (e.key) {
        case 'ArrowDown':
        case 'j':
          next = Math.min(current + 1, count - 1)
          break
        case 'ArrowUp':
        case 'k':
          next = Math.max(current - 1, 0)
          break
        case 'Home':
          next = 0
          break
        case 'End':
          next = count - 1
          break
        case 'Enter':
          e.preventDefault()
          onOpen(current)
          return
        default:
          return
      }
      e.preventDefault()
      setSelected(next)
      // The list takes focus without scrolling, so bring the newly selected row into view.
      const rows = (e.currentTarget as HTMLElement).querySelectorAll('[aria-selected]')
      rows[next]?.scrollIntoView?.({ block: 'nearest' })
    },
    [count, current, onOpen],
  )
  return { selected: current, setSelected, onKeyDown }
}
