import { useEffect, useId, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { CloseIcon } from '../icons.tsx'

interface Props {
  title: string
  onClose: () => void
  closeDisabled?: boolean // while work is under way (a run), the close button waits
  drawer?: boolean // slides in from the right, leaving the screen behind in view
  children: ReactNode
}

// Modal shows a form over the whole window (it is rendered on the page body, so it covers
// the sidebar too). Focus starts on its first field and goes back where it was on closing;
// the page behind does not scroll. The close button (or the form's own Cancel) closes it; a
// click outside does not, so a stray click loses nothing.
export function Modal({ title, onClose, closeDisabled, drawer, children }: Props) {
  const box = useRef<HTMLDivElement>(null)
  const id = useId()
  useEffect(() => {
    const before = document.activeElement as HTMLElement | null
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const first = box.current?.querySelector<HTMLElement>('input, select, textarea') ?? box.current?.querySelector<HTMLElement>('button')
    first?.focus()
    return () => {
      document.body.style.overflow = overflow
      before?.focus?.()
    }
  }, [])
  return createPortal(
    <div className={drawer ? 'sheet-backdrop drawer-backdrop' : 'sheet-backdrop'}>
      <div ref={box} className={drawer ? 'sheet wide drawer' : 'sheet wide card-premium-enhanced'} role="dialog" aria-modal="true" aria-labelledby={id}>
        <div className="side-panel-head">
          <h2 id={id}>{title}</h2>
          <button type="button" className="ghost icon-btn" onClick={onClose} disabled={closeDisabled}
            aria-label={`Close ${title.toLowerCase()}`} title="Close">
            <CloseIcon />
          </button>
        </div>
        {children}
      </div>
    </div>,
    document.body,
  )
}
