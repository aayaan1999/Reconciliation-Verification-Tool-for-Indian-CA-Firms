# Architecture decision records

Each file here records one design decision that has more than one credible option: the context, the decision, and what follows from it. Decisions are made with the `architecture-compare` skill (`.claude/skills/architecture-compare/SKILL.md`) and recorded once the project owner agrees.

## Index

| ADR | Title | Status | Date |
|---|---|---|---|
| [ADR-0001](ADR-0001-engine-network-proof.md) | How the engine proves it has no network code; SQLite driver and excelize | Accepted | 14 October 2026 |
| [ADR-0002](ADR-0002-database-encryption.md) | Database encryption: the SQLite driver's AES-256-XTS instead of SQLCipher | Accepted | 14 October 2026 |
| [ADR-0003](ADR-0003-bundle-poppler.md) | Bundle Poppler pdftotext (GPL) with Tool 1 | Accepted | 14 October 2026 |

## Writing a new ADR

1. Name the file `ADR-NNNN-short-title.md` with the next free number. Numbers are never reused.
2. Use this shape:

   ```markdown
   # ADR NNNN — Title

   - **Status:** Proposed | Accepted (who, date) | Superseded by ADR-NNNN
   - **Date:** day month year
   - **Affects:** CLAUDE.md lines, tech spec sections, specs

   ## Context
   What forces the decision, the options, and the evidence (spikes, measurements).

   ## Decision
   What was chosen, stated so a reviewer can check the code against it.

   ## Consequences
   What becomes easier, what becomes harder, what is checked and by which test.
   ```

3. Add a row to the index above.
4. If the decision changes the design, update `tool1-technical-spec.md` and `CLAUDE.md` in the same commit.
5. An accepted ADR is not edited to reverse it. Write a new ADR that supersedes it, and set the old one's status to "Superseded by ADR-NNNN".
