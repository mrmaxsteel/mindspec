# Plan 034 Review Feedback

**Date**: 2026-02-17
**Reviewer**: Claude (requested by user)
**Plan version**: 0.2

---

## Overall Assessment

The plan is well-structured with good parallelism (034-B/034-C after 034-A), explicit ADR fitness evaluation, and a dogfooding bead that makes adoption real rather than theoretical. The spec revisions since v0.1 addressed all prior review gaps (workspace path migration, LLM fallback, flag semantics, transactional apply, scoped ADR supersession).

**Recommendation**: Approve with one amendment — move `policies.yml` into `.mindspec/` as part of this spec.

---

## Amendment: Move `policies.yml` into `.mindspec/`

### Current state

`architecture/policies.yml` sits at a standalone path outside the documentation tree. It is:
- Machine-readable YAML config (not Markdown documentation)
- Consumed only by context-pack assembly (`internal/contextpack/policy.go`)
- Resolved via `workspace.PoliciesPath()` → `<root>/architecture/policies.yml`
- Manually maintained with no sync validation against ADRs or mode definitions

### Why it should move

1. **It's MindSpec operational state, not project architecture.** The `architecture/` directory name implies it belongs to the *project being built*. But `policies.yml` governs MindSpec's own workflow enforcement — it's in the same category as `.mindspec/state.json`.

2. **The carve-out adds complexity for no value.** The current plan threads `policies.yml` exclusions through 4 beads (034-A, 034-C, 034-E, 034-G). Moving it to `.mindspec/policies.yml` eliminates those carve-outs and makes the migration rule simpler: "everything under `.mindspec/` is MindSpec operational state; everything else is project content."

3. **It simplifies the canonical layout.** Instead of two separate trees (`.mindspec/docs/` + `architecture/`), there's one root for all MindSpec-owned state.

4. **The migration is trivial.** Update `PoliciesPath()` in `workspace.go`, update the hardcoded path in `contextpack/builder.go`, move the file, update references.

### Proposed change

- Canonical location: `.mindspec/policies.yml`
- Remove `architecture/` directory entirely (it only contains `policies.yml`)
- Drop the policies carve-out from beads 034-A, 034-C, 034-E, 034-G
- Add policies path migration to bead 034-C (workspace path migration)

### Impact on canonical layout

```text
.mindspec/
  state.json
  policies.yml          # moved from architecture/policies.yml
  docs/
    core/
    domains/
    adr/
    specs/
    context-map.md
    glossary.md
    index.json
  lineage/
    manifest.json
  migrations/
    <run-id>/
      ...
```

---

## Minor observations

### 1. `policies.yml` reference paths will break

The `reference:` fields in `policies.yml` point to `docs/core/MODES.md`, `docs/core/ARCHITECTURE.md`, `docs/core/CONVENTIONS.md`, and `docs/adr/ADR-0002.md`. After brownfield migration, these paths are archived. Bead 034-G (dogfooding) must update these references to canonical paths — this is already implied by step 4 ("update remaining repository-specific references") but should be called out explicitly.

### 2. Policy drift is a systemic gap

`policies.yml` has no validation keeping it in sync with ADRs, mode definitions, or new commands. This is out of scope for spec 034, but worth a dedicated spec (see below).

### 3. Plan validation ADR checks are too soft

At plan-approve, citing a Superseded or Proposed ADR is a warning, not a blocking error. This means a plan can be approved while citing dead ADRs. This is also out of scope for 034 but should be tightened — see the proposed governance spec below.

---

## Verdict

**Approve with the `policies.yml` → `.mindspec/policies.yml` amendment.** The amendment simplifies the plan (removes 4 carve-outs), makes the canonical layout cleaner, and correctly categorizes policies as MindSpec operational state.

---

## Disposition

- [x] Feedback addressed on 2026-02-17.
- [x] Spec and plan updated to adopt `.mindspec/policies.yml` as canonical policy location.
- [x] Plan updated to include policy `reference:` canonicalization during migration/dogfooding.
