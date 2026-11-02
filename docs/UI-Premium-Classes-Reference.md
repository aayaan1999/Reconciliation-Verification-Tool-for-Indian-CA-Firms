# Premium UI Classes Reference (SPEC-014)

## Quick Reference - CSS Classes Used in ClientFile

This guide documents all premium UI classes applied to the ClientFile.tsx screen.

## Container Classes

### `.header-premium`
**Used on:** Screen header containing client name and details  
**Applied to:** `<div className="screen-head header-premium">`  
**Features:**
- Bottom border: 1px solid #e2e8f0
- H1 styling: 28px bold, dark navy (#000D2F)
- Responsive: Stacks on mobile, 24px font at 480px
- Animation: slideDown 200ms on load

**CSS Variables:**
```css
border-bottom: 1px solid var(--premium-border);
margin-bottom: 24px;
padding-bottom: 16px;
```

---

### `.upload-bar`
**Used on:** Container for "Upload and run" button and options  
**Applied to:** `<div className="upload-bar">`  
**Features:**
- Background: Subtle golden gradient overlay
- Border: 1px solid #e2e8f0
- Border-radius: 16px
- Padding: 16px 20px
- Animation: slideDown 200ms

**CSS Variables:**
```css
background: linear-gradient(135deg, rgba(251, 212, 50, 0.05) 0%, rgba(244, 201, 36, 0.02) 100%);
border: 1px solid var(--premium-border);
border-radius: var(--premium-radius-card);
animation: slideDown var(--premium-duration) var(--premium-ease);
```

---

## Button Classes

### `.btn-golden`
**Used on:** Primary action buttons (Upload, Run checks, Fix)  
**Applied to:**
- `<button className="primary btn-golden">Upload and run</button>`
- `<button type="submit" className="btn-golden">Run checks</button>`
- `<button className="btn-golden">Fix</button>` (in decision panel)

**Features:**
- Background: Golden gradient (135deg, #FBD432 → #f4c924)
- Text color: Dark navy (#000D2F)
- Box shadow: 0 8px 20px rgba(251, 212, 50, 0.3)
- Hover: -3px lift with shadow 0 12px 30px
- Active: -1px lift with shadow 0 6px 16px
- Disabled: 60% opacity
- Transition: 200ms smooth

**CSS:**
```css
background: linear-gradient(135deg, #FBD432 0%, #f4c924 100%);
color: #000D2F;
font-weight: 600;
box-shadow: 0 8px 20px rgba(251, 212, 50, 0.3);
transition: all 200ms cubic-bezier(0.4, 0, 0.2, 1);
```

---

### `.btn-sm-premium`
**Used on:** Secondary buttons (Not an issue, Ask client)  
**Applied to:**
- `<button className="btn-sm-premium">Not an issue</button>`
- `<button className="btn-sm-premium">Ask client</button>`

**Features:**
- Background: White with 1.5px border
- Border color: #e2e8f0
- Padding: 8px 14px
- Font: 13px bold
- Hover: Border #cbd5e1, background light gray
- Active: Light gray background, golden border, golden text
- Transition: 200ms smooth

**CSS:**
```css
padding: 8px 14px;
background: white;
border: 1.5px solid var(--premium-border);
border-radius: 6px;
font-size: 13px;
font-weight: 600;
color: #64748b;
```

---

## Tab Classes

### `.premium-tabs`
**Used on:** Tab list container  
**Applied to:** `<div className="tabs premium-tabs" role="tablist">`  
**Features:**
- Display: Flex with wrap
- Gap: 4px between tabs
- Border-bottom: 2px solid #e2e8f0
- Animation: slideDown 200ms

**CSS:**
```css
display: flex;
flex-wrap: wrap;
gap: 4px;
border-bottom: 2px solid var(--premium-border);
animation: slideDown 200ms var(--premium-ease);
```

---

### `.premium-tab`
**Used on:** Individual tab button  
**Applied to:** `<button className="premium-tab" role="tab">`  
**Features:**
- Border-radius: 6px 6px 0 0 (top only)
- Border-bottom: 3px solid transparent (becomes golden when active)
- Padding: 10px 16px
- Color: Muted gray (#64748b)
- Font-weight: 500
- Hover: Light background (#f8fafc), navy text
- Active: Navy text, 600 weight, golden border
- Focus: 2px golden outline
- Transition: All 200ms

**CSS:**
```css
border: none;
border-radius: 6px 6px 0 0;
border-bottom: 3px solid transparent;
background: transparent;
color: #64748b;
padding: 10px 16px;
margin-bottom: -2px;
font-weight: 500;
transition: all 200ms var(--premium-ease);
```

**Active State:**
```css
.premium-tab[aria-selected='true'] {
  color: #000D2F;
  font-weight: 600;
  border-bottom-color: #FBD432;
  background: white;
}
```

---

## Card Classes

### `.card-premium-enhanced`
**Used on:** Finding cards, decision panels  
**Applied to:**
- `<article className="finding card-premium-enhanced">`
- `<div className="card-premium-enhanced decision">`

**Features:**
- Background: White
- Border: 1px solid #e2e8f0
- Border-left: 4px solid #f87171 (red for fail)
- Border-radius: 16px
- Padding: 20px 24px
- Box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05)
- Animation: slideUp 300ms on load
- Hover: -2px lift, shadow 0 8px 24px
- Focus: Golden border + golden shadow
- Decided state: 60% opacity, left border green, top bar indicator
- Transition: 200ms smooth

**CSS:**
```css
background: white;
border: 1px solid var(--premium-border);
border-left: 4px solid #f87171;
border-radius: 16px;
padding: 20px 24px;
box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
animation: slideUp 300ms var(--premium-ease);
transition: all 200ms var(--premium-ease);
```

**Hover State:**
```css
.card-premium-enhanced:hover {
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
  transform: translateY(-2px);
}
```

**Decided State:**
```css
.card-premium-enhanced.decided {
  border-left-color: #166534;
  opacity: 0.85;
}
.card-premium-enhanced.decided::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: #dcfce7;
  border-radius: 16px 16px 0 0;
}
```

---

## Table Classes

### `.table-premium`
**Used on:** All data tables within findings  
**Applied to:**
- `<table className="grid table-premium">`
- `<table className="grid compact table-premium">`

**Features:**
- Border: 1px solid #e2e8f0
- Border-radius: 16px
- Overflow: Hidden
- Box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05)
- Header background: #f8fafc
- Header font: 12px uppercase, bold
- Row padding: 14px 16px
- Hover: Light background, subtle golden shadow
- Focus: 2px golden outline
- Animations: Staggered 30ms per row

**CSS:**
```css
border: 1px solid var(--premium-border);
border-radius: 16px;
box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
```

**Header:**
```css
.table-premium thead {
  background: #f8fafc;
  border-bottom: 1px solid var(--premium-border);
}
.table-premium th {
  padding: 14px 16px;
  font-size: 12px;
  font-weight: 600;
  color: #475569;
  text-transform: uppercase;
  letter-spacing: 0.4px;
}
```

**Body:**
```css
.table-premium tbody tr {
  border-bottom: 1px solid var(--premium-border);
  transition: all 200ms var(--premium-ease);
}
.table-premium tbody tr:hover {
  background: #f8fafc;
  box-shadow: inset 0 0 0 1px rgba(251, 212, 50, 0.1);
}
```

---

## Sidebar Classes

### `.sidebar-premium`
**Used on:** Finding index sidebar (lists findings by document)  
**Applied to:** `<nav className="desk-index sidebar-premium">`  
**Features:**
- Background: White
- Border: 1px solid #e2e8f0
- Border-radius: 16px
- Padding: 20px
- Box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05)
- Position: Sticky at top: 24px (desktop only)
- Max-height: calc(100vh - 48px)
- Overflow: Auto
- Animation: slideDown 300ms on load
- Responsive: Static at 1100px, hidden at 760px

**CSS:**
```css
position: sticky;
top: 24px;
background: white;
border: 1px solid var(--premium-border);
border-radius: 16px;
padding: 20px;
box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
max-height: calc(100vh - 48px);
overflow-y: auto;
animation: slideDown 300ms var(--premium-ease);
```

**Sidebar Button (finding list item):**
```css
.sidebar-premium button {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 2px 12px;
  width: 100%;
  text-align: left;
  border: none;
  background: transparent;
  padding: 12px 10px;
  cursor: pointer;
  transition: all 200ms var(--premium-ease);
}

.sidebar-premium button[aria-current='true'] {
  background: rgba(251, 212, 50, 0.1);
  border-left: 3px solid #FBD432;
  box-shadow: inset 3px 0 0 #FBD432;
  font-weight: 600;
  color: #000D2F;
  padding-left: 7px;
}
```

---

## Animation Classes

All animations use these CSS variables:
```css
--premium-ease: cubic-bezier(0.4, 0, 0.2, 1)
--premium-duration: 200ms
--premium-duration-slow: 300ms
```

### `slideDown` Animation
```css
@keyframes slideDown {
  from {
    opacity: 0;
    transform: translateY(-20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
```

### `slideUp` Animation
```css
@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
```

---

## Responsive Adjustments

### Tablet (max-width: 768px)
- Premium tabs: 2px gap, reduced padding
- Cards: Maintained 16px padding
- Decision buttons: 7px 12px padding, 12px font

### Mobile (max-width: 640px)
- Header: Stacked, 24px H1
- Tabs: 7px 8px padding, 11px font, hidden labels
- Cards: 12px 16px padding, 3px left border
- Upload bar: Full width, stacked buttons
- Decision buttons: Flex wrap, min-width 100px

### Small Mobile (max-width: 480px)
- Header: 20px H1
- Tabs: 6px padding, 10px font
- Cards: 10px 12px padding, 10px border-radius
- Tables: 10px 12px padding, 11px font

---

## Color Reference

| Purpose | Color | Variable |
|---------|-------|----------|
| Golden accent | #FBD432 | --premium-golden |
| Golden darker | #f4c924 | --premium-golden-dark |
| Primary text | #000D2F | --premium-navy |
| Muted text | #64748b | --premium-text-muted |
| Borders | #e2e8f0 | --premium-border |
| Light backgrounds | #f8f9fb | --premium-bg-light |
| Surface light | #f8fafc | --premium-surface-light |
| Success | #dcfce7 | --premium-success |
| Success text | #166534 | --premium-success-text |
| Error | #fee2e2 | --premium-error |
| Error text | #991b1b | --premium-error-text |

---

## Usage Examples

### Adding Premium Styling to New Components

```tsx
// Header
<div className="screen-head header-premium">
  <h1>Title</h1>
</div>

// Button
<button className="btn-golden">Primary Action</button>

// Card
<article className="card-premium-enhanced">
  Content here
</article>

// Table
<table className="grid table-premium">
  <tbody>...</tbody>
</table>

// Sidebar
<nav className="sidebar-premium">
  Items here
</nav>
```

---

## Accessibility Notes

- All interactive elements have focus states (golden outline + shadow)
- Color is never the only signal (use text labels + colors)
- Animations use hardware acceleration (transform, opacity)
- No animation on `@prefers-reduced-motion` (future enhancement)
- Proper contrast ratios (4.5:1 minimum for text)

---

**Last Updated:** 2026-10-30  
**For:** SPEC-014 Premium UI Design System  
**Version:** 1.0
