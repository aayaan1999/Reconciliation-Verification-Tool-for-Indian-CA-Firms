import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import { ApiError } from './api.ts'
import { emptyDetails } from './clientLists.ts'
import { ClientForm } from './screens/ClientForm.tsx'
import type { ClientDetails } from './types.ts'

// The add and edit client form (ClientForm.tsx) after its layout change: both modes share the
// same sections and footer, required labels carry the required class (the star comes from CSS),
// edit mode shows the existing values, and a refused client shows its reason under the field.
// jsdom does not lay out a page, so the one-column phone layout is checked through the structure
// and class names only (the grid and field-wide classes the phone rule targets); the layout itself
// is only checked in a browser.

const existing: ClientDetails = {
  legal_name: 'Deccan Byte Systems Pvt Ltd', trade_name: 'DBS', pan_last4: '565Z', constitution: 'company',
  date_of_incorporation: '2019-04-01', cin: '', tan: 'PNEA12345B',
  gstins: [{ gstin: '27ZLVCD9565Z1ZF', registration_type: 'regular', primary: true }],
  address: { line1: 'Plot 4', line2: '', city: 'Pune', state: '27', pin: '411001' },
  contact_person: 'R. Iyer', email: 'accounts@example.test', phone: '', accounting_software: 'Tally', notes: 'Synthetic client',
}

function renderForm(mode: 'add' | 'edit', over: Partial<Parameters<typeof ClientForm>[0]> = {}) {
  const onSave = vi.fn(async () => {})
  const onCancel = vi.fn()
  const view = render(
    <ClientForm
      mode={mode}
      initial={mode === 'add' ? emptyDetails() : existing}
      taxYears={['2025-26', '2026-27']}
      onSave={onSave}
      onCancel={onCancel}
      {...over}
    />,
  )
  return { ...view, onSave, onCancel }
}

const form = () => screen.getByRole('form')
const sectionHeadings = () => [...form().querySelectorAll('fieldset.client-section > legend')].map((l) => l.textContent)

describe('the client form, both modes (layout)', () => {
  test('add and edit render the same sections in the same order', () => {
    const add = renderForm('add')
    const addSections = sectionHeadings()
    add.unmount()
    renderForm('edit')
    expect(sectionHeadings()).toEqual(addSections)
    // Three labelled sections (Recon Screen Document v2 §5.2).
    expect(addSections).toEqual(['Identity', 'GST registrations', 'Address, contact and software'])
  })

  test('every section lays its fields out on the shared grid, and the footer is the last row of the form', () => {
    renderForm('add')
    const fieldsets = [...form().querySelectorAll('fieldset.client-section')]
    expect(fieldsets).toHaveLength(3)
    // The GST section has its grid only once a GSTIN is added; the other two always have one.
    for (const fs of fieldsets.filter((f) => f.querySelector('legend')?.textContent !== 'GST registrations')) {
      expect(fs.querySelector('.client-grid')).not.toBeNull()
    }
    const footer = form().querySelector('.client-form-actions')
    expect(footer).not.toBeNull()
    expect(form().lastElementChild).toBe(footer)
    expect(footer).toHaveClass('form-actions')
  })

  test('the phone layout is one column: every grid carries the class the phone rule targets (CSS is not computed in jsdom; a browser checks the layout)', () => {
    renderForm('add')
    expect(form().querySelector('.client-grid')).toHaveClass('client-grid')
    // A field that spans both columns on a tablet or desktop is marked, so the phone rule keeps it whole.
    expect(screen.getByRole('textbox', { name: 'Legal name' }).closest('.field-wide')).not.toBeNull()
  })
})

describe('required fields and the star', () => {
  test('add mode: Legal name, PAN and Tax year carry the required class; the star is not typed into the label', () => {
    renderForm('add')
    for (const label of ['Legal name', 'PAN', 'Tax year']) {
      const el = screen.getByText(label, { selector: 'label' })
      expect(el).toHaveClass('required')
    }
    expect(screen.getByText('Trade name', { selector: 'label' })).not.toHaveClass('required')
    expect(screen.getByText('TAN', { selector: 'label' })).not.toHaveClass('required')
  })

  test('no label has a star typed into its text (the CSS adds it)', () => {
    renderForm('add')
    for (const l of form().querySelectorAll('label')) {
      expect(l.textContent ?? '').not.toContain('*')
    }
    expect(form().querySelector('.required-marker')).toBeNull()
  })

  test('edit mode: the PAN is not required, Legal name still is', () => {
    renderForm('edit')
    expect(screen.getByText('Legal name', { selector: 'label' })).toHaveClass('required')
    expect(screen.getByText(/^PAN \(leave empty to keep the one ending 565Z\)$/, { selector: 'label' })).not.toHaveClass('required')
    expect(screen.queryByText('Tax year', { selector: 'label' })).toBeNull()
  })
})

describe('Save and Cancel', () => {
  test('both modes show Save (named for the mode) and Cancel in the footer; Cancel calls onCancel', () => {
    const add = renderForm('add')
    let footer = within(form()).getByRole('button', { name: 'Save client' }).closest('.client-form-actions') as HTMLElement
    expect(within(footer).getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
    fireEvent.click(within(footer).getByRole('button', { name: 'Cancel' }))
    expect(add.onCancel).toHaveBeenCalledTimes(1)
    add.unmount()

    renderForm('edit')
    footer = within(form()).getByRole('button', { name: 'Save changes' }).closest('.client-form-actions') as HTMLElement
    expect(within(footer).getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
    // Cancel is the footer's first button (left), Save its last (right).
    expect(within(footer).getAllByRole('button').map((b) => b.textContent)).toEqual(['Cancel', 'Save changes'])
  })

  test('add mode: Save stays disabled until Legal name and a 10-character PAN are given', () => {
    renderForm('add')
    const save = screen.getByRole('button', { name: 'Save client' })
    expect(save).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox', { name: 'Legal name' }), { target: { value: 'New Client LLP' } })
    expect(save).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox', { name: 'PAN' }), { target: { value: 'abcde1234f' } })
    expect(screen.getByRole('textbox', { name: 'PAN' })).toHaveValue('ABCDE1234F')
    expect(save).toBeEnabled()
  })

  test('edit mode: Save is enabled while Legal name is filled, and an empty PAN is allowed', () => {
    renderForm('edit')
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
    expect(screen.getByRole('textbox', { name: 'PAN' })).toHaveValue('')
  })
})

describe('edit mode shows the existing values in the same layout', () => {
  test('the saved values appear in their fields', () => {
    renderForm('edit')
    expect(screen.getByRole('textbox', { name: 'Legal name' })).toHaveValue('Deccan Byte Systems Pvt Ltd')
    expect(screen.getByRole('textbox', { name: 'Trade name' })).toHaveValue('DBS')
    expect(screen.getByRole('textbox', { name: 'TAN' })).toHaveValue('PNEA12345B')
    expect(screen.getByLabelText('Date of incorporation')).toHaveValue('2019-04-01')
    expect(screen.getByLabelText('GSTIN 1')).toHaveValue('27ZLVCD9565Z1ZF')
    expect(screen.getByLabelText('Registration type 1')).toHaveValue('regular')
    expect(screen.getByLabelText('Primary GSTIN 1')).toBeChecked()
    expect(screen.getByRole('combobox', { name: 'State' })).toHaveValue('27')
    expect(screen.getByRole('textbox', { name: 'City' })).toHaveValue('Pune')
    expect(screen.getByRole('textbox', { name: 'Contact person' })).toHaveValue('R. Iyer')
    expect(screen.getByRole('textbox', { name: 'Notes' })).toHaveValue('Synthetic client')
  })

  test('a field name keeps its accessible name; the label is the field name (not the star)', () => {
    renderForm('add')
    expect(screen.getByRole('textbox', { name: 'Legal name' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Tax year of the client' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '+ Add GSTIN' })).toBeInTheDocument()
  })
})

describe('a refused save shows the reason under its field', () => {
  test('the problem message appears under the field and the field is marked invalid', async () => {
    const onSave = vi.fn(async () => {
      throw new ApiError('INVALID_CLIENT', 'the check character is wrong', [
        { field: 'tan', code: 'BAD_TAN', message: 'a TAN is four letters, five digits and one letter' },
        { field: 'gstins[0]', code: 'GSTIN_CHECK_DIGIT', message: 'the check character is wrong' },
      ], 422)
    })
    renderForm('add', { onSave })
    fireEvent.change(screen.getByRole('textbox', { name: 'Legal name' }), { target: { value: 'Deccan Byte Systems Pvt Ltd' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'PAN' }), { target: { value: 'ZLVCD9565Z' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Save client' }))
    })
    expect(screen.getByRole('alert')).toHaveTextContent('INVALID_CLIENT')
    const tan = screen.getByRole('textbox', { name: 'TAN' }).closest('.form-group-premium') as HTMLElement
    expect(tan).toHaveClass('invalid')
    expect(tan).toHaveTextContent('a TAN is four letters, five digits and one letter')
  })

  test('a GSTIN problem shows under its own GSTIN row', async () => {
    const existingWithGstin = { ...existing }
    const onSave = vi.fn(async () => {
      throw new ApiError('INVALID_CLIENT', 'refused', [{ field: 'gstins[0]', code: 'GSTIN_CHECK_DIGIT', message: 'the check character is wrong' }], 422)
    })
    render(<ClientForm mode="edit" initial={existingWithGstin} taxYears={[]} onSave={onSave} onCancel={vi.fn()} />)
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    })
    const row = screen.getByLabelText('GSTIN 1').closest('.form-group-premium') as HTMLElement
    expect(row).toHaveClass('invalid')
    expect(row).toHaveTextContent('the check character is wrong')
  })
})

describe('the client drawer (Recon Screen Document v2 §5.2)', () => {
  test('opens as a popup in the middle of the screen; optional fields say so, required ones do not', () => {
    renderForm('add')
    expect(screen.getByRole('dialog', { name: 'Add client' })).not.toHaveClass('drawer')
    const trade = screen.getByText('Trade name', { selector: 'label' }).closest('.form-group-premium') as HTMLElement
    expect(trade).toHaveTextContent('Optional')
    const legal = screen.getByText('Legal name', { selector: 'label' }).closest('.form-group-premium') as HTMLElement
    expect(legal).not.toHaveTextContent('Optional')
  })

  test('PAN, TAN and PIN are checked as typed: a message once the field is full, none for a good value', () => {
    renderForm('add')
    const pan = screen.getByRole('textbox', { name: 'PAN' })
    fireEvent.change(pan, { target: { value: 'ABC' } })
    expect(screen.queryByText(/A PAN is five letters/)).toBeNull()
    fireEvent.blur(pan)
    expect(screen.getByText(/A PAN is five letters/)).toBeInTheDocument()
    fireEvent.change(pan, { target: { value: 'abcde1234f' } })
    expect(screen.queryByText(/A PAN is five letters/)).toBeNull()
    fireEvent.change(screen.getByRole('textbox', { name: 'TAN' }), { target: { value: '1234567890' } })
    expect(screen.getByText(/A TAN is four letters/)).toBeInTheDocument()
    fireEvent.change(screen.getByRole('textbox', { name: 'PIN code' }), { target: { value: '011001' } })
    expect(screen.getByText(/A PIN code is six digits/)).toBeInTheDocument()
  })

  test('a GSTIN shows its state, fills an empty state, and warns when it does not contain the PAN', () => {
    renderForm('add')
    fireEvent.change(screen.getByRole('textbox', { name: 'PAN' }), { target: { value: 'ABCDE1234F' } })
    fireEvent.click(screen.getByRole('button', { name: '+ Add GSTIN' }))
    fireEvent.change(screen.getByLabelText('GSTIN 1'), { target: { value: '27abcde1234f1z5' } })
    expect(screen.getByText('State 27: Maharashtra')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'State' })).toHaveValue('27')
    expect(screen.queryByText(/does not contain the PAN/)).toBeNull()
    fireEvent.change(screen.getByLabelText('GSTIN 1'), { target: { value: '27ZZZZZ9999Z1Z5' } })
    expect(screen.getByRole('status')).toHaveTextContent('This GSTIN does not contain the PAN ABCDE1234F.')
  })

  test('edit mode compares a GSTIN with the PAN ending only', () => {
    renderForm('edit')
    expect(screen.queryByText(/does not contain the PAN/)).toBeNull()
    fireEvent.change(screen.getByLabelText('GSTIN 1'), { target: { value: '27ZLVCD1111Z1ZF' } })
    expect(screen.getByRole('status')).toHaveTextContent('This GSTIN does not contain the PAN ending 565Z.')
  })
})
