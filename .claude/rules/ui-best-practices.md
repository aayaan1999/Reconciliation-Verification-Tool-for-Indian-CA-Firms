---
paths:
  - "web/**"
---

# UI Design Best Practices – Development Guidelines

Read this file before starting any UI work. These are the essential UI design best practices to follow while designing or developing any screen, regardless of the language, framework or technology. Where they meet the project rules in `frontend.md` (offline only, keyboard-first, the UI never decides a finding) and `project-invariants.md`, the project rules win.

## 1. Layout & Structure

1. Maintain consistent alignment across all pages and components.
2. Follow a clear visual hierarchy for headings, subheadings, labels and content.
3. Use a consistent grid system throughout the application.
4. Avoid unnecessary whitespace and excessive spacing.
5. Ensure proper positioning of elements, buttons, forms and containers.
6. Keep page layouts balanced and visually organised.
7. Maintain consistent margins, padding and section spacing.
8. Avoid unnecessary horizontal and vertical scrolling.
9. Keep related information grouped together.
10. Ensure all screens follow a consistent layout structure.

## 2. Navigation & Header

1. Keep navigation simple, intuitive and easy to understand.
2. Maintain consistent header and sidebar behaviour across pages.
3. Avoid unnecessarily fixed or sticky headers.
4. If a sticky header is required, ensure it does not consume excessive screen space.
5. Implement collapsible headers or navigation sections where appropriate.
6. Highlight the currently active navigation item.
7. Ensure users can easily understand where they are within the application.

## 3. Typography & Readability

1. Use readable and professional font families.
2. Maintain consistent font sizes and weights.
3. Establish a clear typography hierarchy.
4. Avoid using too many different font styles.
5. Maintain sufficient contrast between text and background.
6. Ensure labels, placeholders and helper text are readable.
7. Avoid unnecessarily large headings that consume screen space.

## 4. Colours & Visual Consistency

1. Maintain a consistent colour palette throughout the application.
2. Use colours according to their purpose (primary, secondary, success, warning, error).
3. Avoid excessive use of bright or competing colours.
4. Maintain sufficient colour contrast for accessibility.
5. Use consistent button, input, border and background colours.
6. Avoid introducing new colours without a design requirement.
7. Maintain consistency between light and dark themes, if supported.

## 5. Forms & Input Fields

1. Align all form fields properly.
2. Maintain consistent input heights and widths.
3. Group related fields logically.
4. Use clear and meaningful field labels.
5. Place validation messages near the relevant input.
6. Clearly indicate required and optional fields.
7. Avoid unnecessarily long forms.
8. Use multi-column layouts when appropriate.
9. Maintain consistent spacing between fields.
10. Ensure forms work properly on different screen sizes.

## 6. Buttons & Actions

1. Maintain consistent button sizes, shapes and styles.
2. Use clear action-oriented button labels.
3. Establish a visual distinction between primary and secondary actions.
4. Avoid displaying too many buttons in one section.
5. Keep important actions easily accessible.
6. Provide hover, focus, active and disabled states.
7. Prevent accidental repeated submissions.
8. Use confirmation dialogs for destructive actions where appropriate.

## 7. Responsive Design

1. Ensure the application works across desktop, tablet and mobile devices.
2. Use flexible layouts instead of fixed dimensions wherever appropriate.
3. Prevent content overflow and overlapping elements.
4. Ensure navigation adapts to smaller screens.
5. Maintain readable typography across screen sizes.
6. Make buttons and interactive elements touch-friendly.
7. Test important screens at different viewport sizes.

## 8. Tables & Data Presentation

1. Maintain consistent column alignment.
2. Use readable row heights and appropriate cell padding.
3. Avoid overcrowding tables with unnecessary information.
4. Provide sorting and filtering where useful.
5. Highlight important information without excessive colours.
6. Use pagination or controlled scrolling for large datasets.
7. Maintain consistent table headers and action placement.
8. Ensure tables remain usable on smaller screens.

## 9. User Experience (UX)

1. Minimise the number of steps required to complete an action.
2. Provide clear feedback after user interactions.
3. Display meaningful success and error messages.
4. Use loading indicators when processing takes time.
5. Maintain consistency in user interactions across screens.
6. Avoid unnecessary popups and confirmation messages.
7. Provide helpful empty states when no data is available.
8. Make important actions predictable.
9. Ensure users can easily recover from errors.
10. Prioritise usability over unnecessary visual decoration.

## 10. Accessibility

1. Maintain sufficient colour contrast.
2. Support keyboard navigation.
3. Provide visible focus indicators.
4. Use meaningful labels for interactive elements.
5. Do not communicate information through colour alone.
6. Ensure appropriate text sizes.
7. Provide accessible error messages.
8. Follow WCAG accessibility guidelines.

## 11. Visual Design & Modern UI

1. Follow a clean, modern and minimal design approach.
2. Use consistent border radius across components.
3. Avoid excessive shadows, borders and gradients.
4. Use whitespace strategically rather than excessively.
5. Maintain consistent card and container styles.
6. Use icons consistently and avoid unnecessary decorative icons.
7. Use subtle animations and transitions where they improve usability.
8. Avoid visual clutter and unnecessary design elements.
9. Ensure all pages look like parts of the same application.

## 12. Code & Design Implementation

1. Follow reusable component-based design principles.
2. Maintain consistent design tokens for colours, spacing, typography and sizing.
3. Avoid duplicate CSS and unnecessary styling overrides.
4. Keep UI components modular and maintainable.
5. Avoid hardcoded dimensions when flexible layouts are more appropriate.
6. Ensure design changes do not break existing functionality.
7. Follow accessibility and responsive design standards.
8. Maintain consistent UI behaviour across all application modules.
