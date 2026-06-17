# Spec: Parallel bead execution

## Goal
Let mindspec run multiple beads at the same time.

## Impacted Domains
- execution: the bead runner
- scheduling: the new parallel scheduler

## Requirements
1. Beads with no dependency on each other run concurrently.
2. The system runs at most one bead at a time to keep things simple.
3. A bead that fails cancels its dependents, verified by a test with A→B→C where B fails and asserting C is marked blocked and never started.

## Scope
### In Scope
- the bead runner
- the scheduler
### Out of Scope
- distributed execution across machines

## Acceptance Criteria
- [ ] Two independent beads observably overlap in time, asserted by timestamps showing bead B starts before bead A finishes.
- [ ] The scheduler is robust under load.
- [ ] A failing bead's dependents are not started, asserted via the A→B→C fixture above.
