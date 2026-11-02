# Handoff — Client form layout (Add client and Edit client)

| | |
|---|---|
| **From** | implementer |
| **To (next agent)** | test-engineer |
| **Date** | 2026-10-30 |
| **Spec** | none: a UI improvement requested directly, with no spec in `specs/` |
| **Part / criteria covered** | the requested layout items (sections, grid, footer, required marks, states, responsive, offline), listed below |

## What was done

- ClientForm.tsx now lays out both modes the same way: four sections (Identity; GST registrations; Registered address; Contact and software) on a two-column grid that becomes one column at 640px and below.
- Each field uses the shared `Field` from RuleBuilder.tsx (exported for this use; no second field system). The label sits above the control; a required label gets its star from `label.required::after` in index.css, and no star is typed into the text.
- Save and Cancel are one footer row at the bottom of the sheet (`.form-actions` with its existing sticky rule). Cancel is now on the left and Save on the right.
- The server problem for a refused value shows under its field; the error block sits just above the footer.
- index.css: the old `.client-form` and `.sheet .client-form` rules are replaced by one scoped block for the form. The shared `.sheet .form-actions` rule (also used by RuleBuilder) is unchanged.
- New test file web/src/clientForm.test.tsx (13 tests).

## Files changed

| File | Change (added / edited / removed) | Why |
|---|---|---|
| web/src/screens/ClientForm.tsx | edited (rewritten markup; state, handlers, API calls and payloads unchanged) | new layout, shared Field, footer, star from CSS |
| web/src/screens/RuleBuilder.tsx | edited (one word: `function Field` is now `export function Field`) | ClientForm reuses the field pattern |
| web/src/index.css | edited (client form rules replaced in the Forms section and the sheet section; the phone rule for `.sheet .form-actions` untouched) | scoped layout for the form |
| web/src/clientForm.test.tsx | added | structure, required marks, Save and Cancel, edit values, server problem under its field |
| docs/Tool1-File-Guide.docx | edited (ClientForm row description updated; row added for clientForm.test.tsx) | file guide for the new test file |
| docs/agents/handoff-client-form-layout.md | added | this note |

Not touched: web/src/screens/Login.tsx (changed by someone else; left as it is), the running server, build/, any commit.

New files to add to `docs/Tool1-File-Guide.docx`: done (clientForm.test.tsx). The handoff note is in docs/agents.
New packages classified in `internal/archtest/archtest_test.go`: none.
New rules added to `testdata/testpack_nimbus/score.yaml`: none.

## Label and markup changes

- No field label or accessible name changed. Every input keeps its `aria-label` (for example "Legal name", "PAN", "GSTIN 1", "Tax year of the client"), so the names tests use are the same.
- Legend "Registered address and contact" is split into two sections: "Registered address" and "Contact and software". The fields moved between them only by section; their names and order are the same.
- Save button: the inline gradient style is removed; the button now uses the existing `premium-golden` class (index.css `button.premium-golden`, same gold gradient).
- Cancel: `secondary-button` class; order in the footer is Cancel then Save.
- Required star: `<span class="required-marker">*</span>` inside the label is removed; the label has `class="required"` and the star comes from CSS.

## Commands run

| Command | Result (exit code, key output) |
|---|---|
| `npx vitest run src/clientForm.test.tsx src/part6.test.tsx` | 30 of 30 passed (before the final test edits) |
| `npm test` (in web/) | exit 0: Test Files 12 passed (12), Tests 127 passed (127) |
| `npm run lint` (in web/) | exit 0: one warning, pre-existing, in src/screens/Rules.tsx:389 (react set-state-in-effect); no errors |
| `npm run build` (in web/) | exit 0: `tsc -b && vite build` built; chunk-size warning only (pre-existing) |
| `go test -count=1 -tags ui ./web/` (repo root) | ok (embedded dist changed) |

No timing failure appeared, so no rerun was needed.

## Visual check (headless Edge, scratch server)

A second build of recon-server with the UI embedded was run on 127.0.0.1:8091 with a scratch database in the scratchpad (one synthetic client, Deccan Byte Systems Pvt Ltd). Headless Edge was driven over DevTools. The server on 8080 was not touched and still serves the older UI build. Both processes started for this check were stopped afterwards.

Measured in the browser (computed styles):

| Width | Add client grid | Document scroll width | Footer | Notes |
|---|---|---|---|---|
| 1280 x 900 | 2 columns (382.5px each) | equals viewport (no horizontal scroll) | Cancel and Save inside the viewport, after scrolling to bottom too | gutter 200px each side (880px sheet) |
| 820 x 1000 | 2 columns (336.5px each) | equals viewport | in view | |
| 390 x 844 | 1 column (326px) | equals viewport | in view after scroll | sheet fills screen; content 16px in from each edge |

Also measured: every text input and select is 40px tall; the Legal name label has no star in its text and the star comes from CSS (`content: " *"`); edit mode shows the existing values, and the PAN label keeps its "(leave empty to keep the one ending 565Z)" text. GST rows were checked with two added rows: fields on a grid, then Primary and Remove on one line, with a divider between rows.

Screenshots (in the scratchpad): desktop-add-top.png, desktop-edit-bottom.png, tablet and phone variants, phone-add-top.png, phone-edit-bottom.png, desktop-gst.png.

## Acceptance (the requested items)

| Item | Status | Evidence |
|---|---|---|
| Both modes render the same sections in the same order | met | clientForm.test.tsx "add and edit render the same sections in the same order" |
| Logical sections with headings, clear separation | met | four fieldsets with legends; browser screenshots |
| Consistent grid; one column on phone | met in browser; structure tested in jsdom | computed grid columns above; test checks grid and field-wide classes |
| Labels above fields; consistent control heights | met | browser: all controls 40px |
| Footer with Save and Cancel, kept in view | met | browser: footer visible after scroll to bottom, both widths |
| Required star via class, not text | met | test "add mode: Legal name, PAN and Tax year carry the required class" and "no label has a star typed into its text" |
| Save disabled logic unchanged | met | test "Save stays disabled until Legal name and a 10-character PAN" |
| Edit mode shows existing values | met | test "the saved values appear in their fields" |
| Server problem under its field | met | test "the problem message appears under the field" and the GSTIN row test |
| Offline only, no remote assets | met (no new asset or font added) | the CSS adds none |
| 16px side gutter on phone, no horizontal scroll | met in browser | measured sheet and document widths |

## Invariant check

- Findings show both values, the difference and both sources; no "should": not affected (form only)
- Money in integer paise, no floats: not affected (no money field changed)
- No clock, randomness or map-order dependence: yes (no change)
- No network import on the engine side: not affected
- No real client data added: yes (synthetic names, PANs and GSTINs only; the scratch password was a throwaway value in the scratchpad)

## Open issues

| Issue | Severity (blocking / not blocking) | Who decides (implementer / developer / CA) |
|---|---|---|
| The one-column rule and the 16px gutter are verified in a browser only (jsdom has no layout). The test checks the classes only. | not blocking | developer (accept the browser check) |
| The running server on 127.0.0.1:8080 still serves the older UI build. To see the change there, rebuild with `-tags ui` and restart it; not done here, since the server was not to be replaced. | not blocking | developer |
| Dark mode, Firefox and Safari were not checked. | not blocking | developer |
| `.premium-golden-button` (used by the Add client button on the Season board) has no CSS rule anywhere; left as it is, outside this change. | not blocking | developer |
| index.css repeats several `.form-group-premium` and `.sheet-backdrop` blocks; not cleaned up here. | not blocking | developer |
| Only the sheet's hover lift (`.card-premium-enhanced:hover`) is shared with other sheets; not changed. | not blocking | developer |

## Next agent

Run `npm test`, `npm run lint` and `npm run build` in web/ again, then `go test -count=1 -tags ui ./web/`. For the layout, open the form in a browser at 1280, 820 and 390 widths, and check: one column at 390, the footer in view after scrolling, no horizontal scroll. Do not rely on the jsdom tests for layout.
