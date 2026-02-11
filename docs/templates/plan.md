---
status: Draft
spec_id: <NNN-slug>
version: "0.1"
last_updated: YYYY-MM-DD
# Fields below are populated on approval:
# approved_at: YYYY-MM-DDTHH:MM:SSZ
# approved_by: <human>
# approved_sha: <git commit SHA>
# bead_ids:
#   - beads-xxx      # spec bead
#   - beads-xxx.1    # first implementation bead
# adr_citations:
#   - id: ADR-NNNN
#     sections: ["<relevant sections>"]
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
