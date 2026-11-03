// Inline SVG icons: the UI loads nothing from the internet. Each is decorative
// (aria-hidden); the text next to it carries the meaning.
import type { ReactNode } from 'react'

function Svg({ size = 16, children }: { size?: number; children: ReactNode }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"
      strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">
      {children}
    </svg>
  )
}

export const LedgerIcon = ({ size = 22 }: { size?: number }) => (
  <Svg size={size}>
    <rect x="2.5" y="1.75" width="11" height="12.5" rx="1.5" />
    <path d="M5.5 5h5M5.5 8h5M5.5 11h3" />
    <path d="M2.5 4.5h1M2.5 8h1M2.5 11.5h1" />
  </Svg>
)

export const SearchIcon = () => (
  <Svg size={14}>
    <circle cx="7" cy="7" r="4.25" />
    <path d="m10.25 10.25 3.25 3.25" />
  </Svg>
)

export const SunIcon = () => (
  <Svg>
    <circle cx="8" cy="8" r="3" />
    <path d="M8 1.5v1.5M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1 1M11.6 11.6l1 1M3.4 12.6l1-1M11.6 4.4l1-1" />
  </Svg>
)

export const MoonIcon = () => (
  <Svg>
    <path d="M13.5 9.5A5.5 5.5 0 0 1 6.5 2.5a5.5 5.5 0 1 0 7 7Z" />
  </Svg>
)

export const AutoIcon = () => (
  <Svg>
    <circle cx="8" cy="8" r="5.75" />
    <path d="M8 2.25v11.5" />
    <path d="M8 2.25a5.75 5.75 0 0 1 0 11.5Z" fill="currentColor" stroke="none" />
  </Svg>
)

export const KeyboardIcon = () => (
  <Svg>
    <rect x="1.5" y="4" width="13" height="8" rx="1.5" />
    <path d="M4 6.5h.01M6.5 6.5h.01M9 6.5h.01M11.5 6.5h.01M5 9.5h6" />
  </Svg>
)

export const CheckIcon = () => (
  <Svg size={14}>
    <path d="m3 8.5 3 3 7-7" />
  </Svg>
)

export const CircleIcon = () => (
  <Svg size={14}>
    <circle cx="8" cy="8" r="5" strokeDasharray="2 2" />
  </Svg>
)

export const DocIcon = () => (
  <Svg size={14}>
    <path d="M4 1.75h5.5L12.5 4.75v9.5H4Z" />
    <path d="M9.5 1.75v3h3" />
  </Svg>
)

export const UploadIcon = () => (
  <Svg>
    <path d="M8 10.5V2.5M5 5.5l3-3 3 3" />
    <path d="M2.5 10.5v2.25c0 .4.35.75.75.75h9.5c.4 0 .75-.35.75-.75V10.5" />
  </Svg>
)

export const CloseIcon = () => (
  <Svg size={14}>
    <path d="m4 4 8 8M12 4l-8 8" />
  </Svg>
)

export const BoardIcon = () => (
  <Svg size={18}>
    <rect x="2" y="2" width="5" height="5" rx="1" />
    <rect x="9" y="2" width="5" height="5" rx="1" />
    <rect x="2" y="9" width="5" height="5" rx="1" />
    <rect x="9" y="9" width="5" height="5" rx="1" />
  </Svg>
)

export const ListIcon = () => (
  <Svg size={18}>
    <path d="m2 4 1 1 2-2M2 8.5l1 1 2-2M2 13l1 1 2-2M7.5 4h6.5M7.5 8.5h6.5M7.5 13h6.5" />
  </Svg>
)

export const BookIcon = () => (
  <Svg size={18}>
    <path d="M3 2.5h7.5a2 2 0 0 1 2 2v9H5a2 2 0 0 1-2-2Z" />
    <path d="M3 11.5a2 2 0 0 1 2-2h7.5M6 5.5h4" />
  </Svg>
)

export const UsersIcon = () => (
  <Svg size={18}>
    <circle cx="6" cy="5.5" r="2.5" />
    <path d="M1.5 13.5a4.5 4.5 0 0 1 9 0M10.5 3.2a2.5 2.5 0 0 1 0 4.6M12 9.6a4.5 4.5 0 0 1 2.5 3.9" />
  </Svg>
)

// FormatsIcon is the rail icon of the Formats screen: a file with the lines it is read by.
export const FormatsIcon = () => (
  <Svg size={18}>
    <path d="M3.5 1.75h6L12.5 4.75v9.5h-9Z" />
    <path d="M9.5 1.75v3h3" />
    <path d="M5.5 8h5M5.5 10.5h5M5.5 13h2.5" />
  </Svg>
)

export const ChevronIcon = ({ dir }: { dir: 'left' | 'right' | 'up' }) => (
  <Svg size={14}>
    <path d={dir === 'left' ? 'm10 3-5 5 5 5' : dir === 'right' ? 'm6 3 5 5-5 5' : 'm3 10 5-5 5 5'} />
  </Svg>
)

export const SignOutIcon = () => (
  <Svg>
    <path d="M6 2.5H3.5a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1H6M10.5 11 13.5 8l-3-3M13.5 8H6" />
  </Svg>
)

export const SortIcon =({ dir }: { dir: 'asc' | 'desc' | null }) => (
  <Svg size={12}>
    {dir === 'asc' ? <path d="m4 10 4-4 4 4" /> : dir === 'desc' ? <path d="m4 6 4 4 4-4" /> : <path d="m5 6 3-3 3 3M5 10l3 3 3-3" opacity="0.5" />}
  </Svg>
)

// Icons of the Formats screen's tabs.
export const SlidersIcon = () => (
  <Svg>
    <path d="M2.5 4.5h6M11.5 4.5h2M2.5 11.5h2M7.5 11.5h6" />
    <circle cx="10" cy="4.5" r="1.5" />
    <circle cx="6" cy="11.5" r="1.5" />
  </Svg>
)

export const GapIcon = () => (
  <Svg>
    <path d="M8 2 14.25 13H1.75Z" />
    <path d="M8 6.5v3M8 11.5h.01" />
  </Svg>
)

export const UnknownFileIcon = () => (
  <Svg>
    <path d="M3.5 1.75h6L12.5 4.75v9.5h-9Z" />
    <path d="M9.5 1.75v3h3" />
    <path d="M6.6 8.2a1.4 1.4 0 1 1 1.9 1.3c-.4.2-.5.5-.5.9M8 12h.01" />
  </Svg>
)

// MoreIcon is the three dots of a row's actions menu.
export const MoreIcon = () => (
  <Svg size={16}>
    <circle cx="3" cy="8" r="1.1" fill="currentColor" stroke="none" />
    <circle cx="8" cy="8" r="1.1" fill="currentColor" stroke="none" />
    <circle cx="13" cy="8" r="1.1" fill="currentColor" stroke="none" />
  </Svg>
)
