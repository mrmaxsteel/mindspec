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
work_chunks:
  - id: 1
    title: "<Short title for first chunk>"
    scope: "<Files or components this chunk delivers>"
    verify:
      - "<Specific, testable verification step>"
    depends_on: []
  - id: 2
    title: "<Short title for second chunk>"
    scope: "<Files or components>"
    verify:
      - "<Verification step>"
    depends_on: [1]
# Machine-generated metadata (written on plan approval):
# The spec-lifecycle molecule tracks bead IDs via state.json stepMapping
---

# Plan: Spec <NNN> — <Title>

**Spec**: [spec.md](spec.md)

---

## Bead <NNN>-A: <Short title>

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

## Bead <NNN>-B: <Short title>

**Scope**: <What this bead delivers>

**Steps**:
1. <Step 1>
2. <Step 2>
3. <Step 3>

**Verification**:
- [ ] <Specific, testable criterion>
- [ ] <Specific, testable criterion>

**Depends on**: <NNN>-A

---

## Dependency Graph

```
<NNN>-A (<short description>)
  └── <NNN>-B (<short description>)
```
