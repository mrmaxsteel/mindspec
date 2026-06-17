# Spec: Improve export reliability

## Goal
Improve export reliability so exports are more reliable.

## Impacted Domains
- execution: the export runner
- core: config

## Requirements
1. Enable resumable exports after a crash.
2. Exports must finish within a reasonable time.
3. When an export fails partway, the next run must not duplicate already-exported rows, verified by exporting 1000 rows, killing the process at row 500, resuming, and asserting the output has exactly 1000 distinct rows.

## Scope
### In Scope
- the export runner
- the resume checkpoint file
### Out of Scope
- changing the export file format

## Acceptance Criteria
- [ ] Resumable exports behave as expected.
- [ ] A killed-and-resumed export produces exactly the same row set as an uninterrupted export, asserted byte-for-byte on a 1000-row fixture.
- [ ] Performance is acceptable.
