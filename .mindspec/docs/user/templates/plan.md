---
status: Draft
spec_id: <NNN-slug>
version: "0.1"
last_updated: YYYY-MM-DD
# Fields below are populated on approval:
# approved_at: YYYY-MM-DDTHH:MM:SSZ
# approved_by: <human>
# bead_ids:
#   - beads-xxx      # spec bead
#   - beads-xxx.1    # first implementation bead
# adr_citations:
#   - id: ADR-NNNN
#     sections: ["<relevant sections>"]
# Structured inter-bead dependencies (consumed at `plan approve`).
# Declare ONE entry per `## Bead <N>` section below, in declaration order.
# Mapping: chunk `id N` (1-based) → bead_ids[N-1] (the Nth `## Bead` section);
# `depends_on: [M]` makes bead_ids[N-1] depend on bead_ids[M-1]. The ids MUST be
# the contiguous set 1..K where K = the number of `## Bead` sections (a gap,
# duplicate, count mismatch, or out-of-range depends_on target is rejected).
# title/scope/verify are human-readable only — the parser reads id, depends_on,
# and key_file_paths. `key_file_paths` is the DECLARED source for that bead's
# `## Key File Paths` context surface (chunk id N → bead_ids[N-1].metadata.file_paths);
# omit or use [] when none — the surface is then empty (non-gating enrichment).
work_chunks:
  - id: 1
    title: "<Short title for first chunk>"
    scope: "<Files or components this chunk delivers>"
    verify:
      - "<Specific, testable verification step>"
    depends_on: []
    key_file_paths:
      - internal/foo/foo.go
  - id: 2
    title: "<Short title for second chunk>"
    scope: "<Files or components>"
    verify:
      - "<Verification step>"
    depends_on: [1]
    key_file_paths:
      - internal/bar/bar.go
      - cmd/mindspec/bar.go
# Machine-generated metadata (written on plan approval):
# The spec-lifecycle molecule tracks bead IDs via state.json stepMapping
---

# Plan: Spec <NNN> — <Title>

**Spec**: [spec.md](spec.md)

---

## Bead 1: <Short title>

> Section #1 → `work_chunks` id 1 → bead_ids[0].

**Scope**: <What this bead delivers — one slice of value>

**Steps**:
1. <Step 1>
2. <Step 2>
3. <Step 3>

**Verification**:
- [ ] <Specific, testable criterion>
- [ ] <Specific, testable criterion>

**Depends on**: nothing

---

## Bead 2: <Short title>

> Section #2 → `work_chunks` id 2 → bead_ids[1]; `depends_on: [1]` above.

**Scope**: <What this bead delivers>

**Steps**:
1. <Step 1>
2. <Step 2>
3. <Step 3>

**Verification**:
- [ ] <Specific, testable criterion>
- [ ] <Specific, testable criterion>

**Depends on**: Bead 1

---

## Dependency Graph

```
Bead 1 (<short description>)
  └── Bead 2 (<short description>)
```
