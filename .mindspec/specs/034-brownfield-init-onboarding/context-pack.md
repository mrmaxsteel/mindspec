# Context Pack

- **Spec**: 034-brownfield-init-onboarding
- **Mode**: plan
- **Commit**: ece74c52b240d056762a4b0af7489b012fd85455
- **Generated**: 2026-02-17T16:49:39Z

---

## Goal

Enable teams to adopt MindSpec in an existing repository with one command that:
- discovers and analyzes existing Markdown docs,
- produces a canonical MindSpec documentation corpus in a deterministic layout, and
- archives legacy docs with full provenance and reproducibility.

## Impacted Domains

- **core**
- **context-system**
- **workflow**

## Applicable Policies

| ID | Severity | Description | Reference |
|:---|:---------|:------------|:----------|
| plan-mode-no-code | error | In Plan Mode, only Beads entries, plan documents, ADR proposals, and documentation may be modified. Code changes are forbidden until the plan is approved. | docs/core/MODES.md#plan-mode |
| spec-required | error | Every functional change must refer to a spec in docs/specs/ | — |
| doc-sync-required | warning | Changes to core logic must be accompanied by updates to docs/core/, docs/domains/, or docs/features/. Done includes doc-sync. | — |
| adr-divergence-gate | error | If implementation or planning detects that an accepted ADR blocks progress or is unfit, the agent must stop, inform the user, and present divergence options. A new superseding ADR requires human approval. | docs/core/MODES.md#implementation-mode |
| plan-must-cite-adrs | warning | Plans and implementation beads must cite the ADRs they rely on. Uncited ADR reliance is a policy violation. | docs/core/ARCHITECTURE.md#adr-lifecycle |
| domain-operations-require-approval | error | Adding, splitting, or merging domains requires explicit human approval and must produce an ADR. | docs/core/ARCHITECTURE.md#domains |
| spec-declares-impacted-domains | warning | Every spec must declare its impacted domains and relevant ADR touchpoints. | docs/core/MODES.md#spec-mode |
| beads-concise-entries | warning | Beads entries must remain concise and execution-oriented. Long-form specs, ADRs, and domain docs live in the documentation system, not in Beads. | docs/adr/ADR-0002.md |
| beads-active-workset | warning | Keep only active and near-term issues open in Beads. Regularly clean up completed work. Rely on git history + docs for archival traceability. | docs/adr/ADR-0002.md |
| clean-tree-before-transition | error | Working tree must be clean (no uncommitted changes) before starting new work, picking up a bead, or switching modes. If dirty: commit or revert. Never auto-stash. | docs/core/CONVENTIONS.md#clean-tree-rule |
| milestone-commit-at-transition | error | Mode transitions must produce a milestone commit: spec(<bead-id>) for Spec→Plan, plan(<bead-id>) for Plan→Implement, impl(<bead-id>) for Implement→Done. .beads/ changes must be co-committed. | docs/core/CONVENTIONS.md#milestone-commits |

---

## Provenance

| Source | Section | Reason |
|:-------|:--------|:-------|
| architecture/policies.yml | Policies | Policies applicable to mode "plan" |
