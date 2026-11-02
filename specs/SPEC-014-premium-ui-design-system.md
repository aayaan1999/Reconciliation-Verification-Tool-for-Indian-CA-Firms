# SPEC-014 — Premium UI Design System

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 6 |
| **Depends on** | SPEC-013 (role-based access), SPEC-012 (admin tab), SPEC-010 (rule management) |
| **Requirements** | §11.2 (three working screens: Season board, Client file, Worklist plus Rules), premium aesthetic with sophisticated design |
| **Technical spec** | §11 (UI screens), cosmos.studio design principles, responsive HTML/CSS (no frameworks) |
| **Code / content paths** | `web/src/screens/` (13 premium design screens), inline CSS styling, system fonts only |

## Goal

Deliver a complete premium, modern, sophisticated UI system across all 13 application screens using cosmos.studio-inspired design principles, with bold golden (#FBD432) and dark navy (#000D2F) accents, smooth animations, balanced whitespace, refined cards, subtle borders, and meaningful micro-interactions that work flawlessly on desktop, tablet and mobile.

## Scope

### 1. Design Language & Aesthetics

- **Color Scheme:** 
  - Primary accent: #FBD432 (golden gradient to #f4c924)
  - Primary text: #000D2F (dark navy)
  - Neutral backgrounds: #f8f9fb to #f0f4f8 (light gradients)
  - Subtle borders: #e2e8f0
  - Secondary text: #64748b (slate)

- **Typography:**
  - System fonts only: `-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif`
  - No external font libraries (offline compliance)
  - Font weights: 400 (regular), 500 (medium), 600 (semibold), 700 (bold)
  - Clear hierarchy: H1 (28px), H2 (20px), H3 (14px), body (14px), labels (12px)

- **Spacing & Layout:**
  - Base unit: 8px grid
  - Padding: 20px (mobile), 24px (desktop), 32px (cards)
  - Gap between elements: 12px, 16px, 20px, 24px
  - Responsive grid: 1 column (mobile), 2 columns (tablet), 4 columns (desktop)

- **Cards & Surfaces:**
  - Border radius: 10px (inputs, badges), 16px (cards, modals), 2px (grid cells)
  - Borders: 1px solid #e2e8f0 (subtle)
  - Box shadows: `0 4px 12px rgba(0, 0, 0, 0.05)` (cards), `0 20px 50px rgba(0, 0, 0, 0.15)` (modals)
  - Background gradients on panels and headers
  - White cards on gradient backgrounds

- **Animations & Interactions:**
  - Duration: 200ms (primary), 300ms (complex)
  - Easing: `cubic-bezier(0.4, 0, 0.2, 1)` (smooth)
  - Hover effects: translateY(-2px), shadow increase, color change
  - Focus states: golden border + 3px golden shadow with 15% opacity
  - Staggered animations for lists (50ms, 100ms, 150ms delays)
  - Fade-in on load, slide-up from bottom, slide-down from top

### 2. Core Screens (13 Total)

#### A. Published Screens (6)
1. **Login Screen** — Hero section left, form right, golden buttons, setup mode, error/success messaging
2. **Season Board** — 6 metric cards, search bar, 12-row client table, pagination controls
3. **Client File (v2)** — Header with client details, upload panel, check-type tabs, findings view with sidebar index
4. **Worklist** — Sticky side panel (year comparison, month grid), 8-row table, classification inputs, filter buttons
5. **Rules Screen** — 7 filter controls, 9-row table, status badges, edit action buttons, pagination
6. **Edit Client** — Form sections (Basic, Tax, Contact, Notes), required field markers, save/cancel buttons

#### B. New Screens to Add (7)
7. **Add Rule Screen** — Rule Builder form with 5 sections (Who, Where, Figures, Check Logic, Fixtures), sidebar guide
8. **Admin Tab (Users)** — User management table (8+ users), role dropdowns, status badges, search/filters, add button
9. **Disable Rule Modal** — Warning-styled modal, rule info, reason textarea, red disable button, ⚠️ icon
10. **Rule Approval Modal** — Golden-styled modal, rule summary, fixtures preview, partner name input, fixtures validation
11. **Edit Rule Screen** — Similar to Add Rule, version selector, compare button, reset option, disable in sidebar
12. **Compare Versions Screen** — Side-by-side version comparison, yellow highlights on changes, switch button
13. **Client File Summary Card** — Header card with client details, last run info, findings summary (critical/major counts)

### 3. Component Library (Reusable Patterns)

- **Buttons:**
  - Golden gradient buttons (primary actions): gradient + shadow + hover lift + color change
  - Secondary buttons (cancel, back): light background + border + hover shade
  - Danger buttons (disable, delete): red gradient + white text + red shadow
  - Icon buttons: no background + color on hover

- **Forms:**
  - Input/textarea: 11px padding, 1.5px borders, 200ms transition
  - Focus state: golden border + semi-transparent golden shadow box
  - Hover state: border color darkens (#cbd5e1)
  - Required field marker: red asterisk
  - Labels: 13px semibold, 64748b color
  - 2-column grid on desktop, 1 column on mobile

- **Tables:**
  - Header: #f8fafc background, uppercase labels, 12px font
  - Rows: 14px padding, 1px divider, hover highlight (#f8fafc)
  - Status badges: colored backgrounds (green active, yellow testing, grey disabled)
  - Action buttons: small (6px padding, 12px font), white background + border
  - Pagination: items selector, page range, prev/next buttons, disabled states

- **Badges & Tags:**
  - Status: Active (green), Testing (yellow), Disabled (grey), Draft (blue)
  - Types: Purple background + white text
  - Kinds: Blue background + darker text
  - Required marker: Red asterisk

- **Modals:**
  - Centered, max 500px wide
  - Overlay: 0.3 opacity + blur(4px)
  - Slide-up animation (300ms)
  - Header: icon + title + subtitle
  - Body: form or info
  - Footer: cancel + action buttons

- **Animations:**
  - Slide-down: header, hero section (300ms from top)
  - Slide-up: rows, cards (300ms staggered from bottom)
  - Fade-in: content, modals (300ms)
  - Hover lift: translateY(-2px), shadow increase
  - Pulse: golden badges on interaction

### 4. Responsive Design

- **Mobile (< 640px):**
  - Single column layouts
  - Reduced padding (20px)
  - Full-width buttons and inputs
  - Stacked sections
  - Touch-friendly tap targets (44px minimum)
  - Simpler table layouts or horizontal scroll

- **Tablet (640px - 1000px):**
  - 2-column grids
  - Side panels become main content
  - Flexible spacing
  - Maintained typography hierarchy

- **Desktop (> 1000px):**
  - Full grid layouts (3-4 columns)
  - Sticky side panels
  - Optimal spacing and whitespace
  - All interactions available

### 5. Accessibility & Semantics

- Semantic HTML (form, label, table, button, etc.)
- ARIA labels where needed
- Color contrast: 4.5:1 minimum for text
- Focus visible: golden border + shadow
- Keyboard navigation: Tab, Enter, Arrow keys
- No reliance on color alone (use icons + text)

### 6. Performance & Dependencies

- **Zero external libraries:** inline CSS, no framework
- **No external resources:** system fonts only, no CDN, no web fonts
- **Optimization:**
  - Inline CSS in HTML (production optimization via build)
  - Minimal JavaScript (form interaction, state only)
  - Hardware acceleration: transform/opacity for animations
  - Lazy load images/icons (if any added later)

## Design Decisions

1. **Color Scheme:** #FBD432 and #000D2F chosen to match cosmos.studio aesthetic, premium feel with high contrast
2. **Animations:** 200ms duration chosen for snappy but not jarring interactions; staggered delays create visual hierarchy
3. **Responsive:** Mobile-first approach ensures accessibility on all devices; tested at 320px, 768px, 1200px
4. **System Fonts:** Offline compliance and performance; no font loading required
5. **Card-Based:** Premium feel achieved through subtle shadows, rounded corners, and refined spacing
6. **Sidebar Patterns:** Sticky side panels on desktop, grid layout on tablet, full-width on mobile

## Acceptance Criteria

- [ ] AC-1 — All 13 screens are implemented with premium design language (golden + dark navy, smooth animations, refined cards)
- [ ] AC-2 — All screens are fully responsive: render correctly at 320px (mobile), 768px (tablet), 1200px+ (desktop)
- [ ] AC-3 — All buttons, inputs, and interactive elements have smooth 200ms transitions and hover/focus states
- [ ] AC-4 — Color contrast meets WCAG AA standard (4.5:1 minimum for normal text)
- [ ] AC-5 — All screens use system fonts only; no external font libraries or CDN resources
- [ ] AC-6 — Staggered animations on list items work smoothly (50ms, 100ms, 150ms delays)
- [ ] AC-7 — Modals have backdrop blur, slide-up animation, and proper overlay with 0.3 opacity
- [ ] AC-8 — Golden gradient buttons used consistently for primary actions (Add, Save, Approve, Run)
- [ ] AC-9 — Forms show required field markers (*), focused inputs have golden border + shadow
- [ ] AC-10 — All tables have sticky headers, hover states, and proper pagination controls
- [ ] AC-11 — Status badges (Active/Testing/Disabled/Draft) color-coded consistently across all screens
- [ ] AC-12 — Side panels are sticky on desktop (position: sticky, top: 24px), static on mobile
- [ ] AC-13 — Modals are centered, have proper close button or cancel action, use warning/success icons
- [ ] AC-14 — Client file summary card shows last run info, findings summary, next actions clearly
- [ ] AC-15 — Edit/compare screens have version selectors and change highlighting (yellow)
- [ ] AC-16 — All screens tested and working in Chrome, Firefox, Safari (latest versions)
- [ ] AC-17 — No accessibility issues: keyboard navigation works, ARIA labels present where needed
- [ ] AC-18 — All files documented in `docs/Tool1-File-Guide.docx` with plain-language descriptions

## Verification

- Manual testing: Open each screen in browser (Chrome, Firefox, Safari) at 320px, 768px, 1200px viewports
- Responsive testing: Use browser DevTools to verify media queries and responsive behavior
- Animation testing: Verify smooth 200ms transitions on hover, focus, and list item stagger
- Accessibility: Use browser accessibility inspector to verify ARIA labels, color contrast, keyboard navigation
- Performance: Check DevTools for fast paint/composite times, no jank on animations
- Offline: Verify all fonts and resources load without internet (system fonts only)

## Completion Record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
