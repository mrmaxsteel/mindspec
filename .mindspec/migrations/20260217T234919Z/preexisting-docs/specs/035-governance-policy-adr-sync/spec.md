# Spec 035-governance-policy-adr-sync: Governance Automation — Policy Sync, ADR Gates, and Domain Lifecycle Checks

## Goal

Close the governance gaps in the MindSpec formula by:
- keeping `policies.yml` in sync with accepted ADRs and mode definitions,
- hardening ADR citation checks from advisory warnings to blocking gates,
- adding an explicit "do we need new/changed ADRs?" gate at spec-approve time,
- adding an explicit "should we add/split/merge domains?" gate at spec-approve time, and
- formalizing `policies.yml` structure with a JSON Schema for validation and editor support.

## Background

The MindSpec lifecycle currently treats governance artifacts (policies, ADR citations, domain boundaries) as advisory context rather than enforced constraints. Five concrete gaps exist:

### 1. Policy drift

`policies.yml` is manually maintained. When a new ADR introduces a policy (e.g., ADR-0002 introduced `beads-concise-entries`), someone must hand-edit the YAML file. There is no validation that policies stay current with accepted ADRs, no check that `reference:` paths resolve to real files, and no warning when a policy references a superseded ADR.

### 2. No schema for `policies.yml`

The parser (`contextpack.ParsePolicies`) does bare YAML unmarshalling with no structural validation. There is no enforcement of valid `severity` values (`error`/`warning`), no enforcement of valid `mode` values (must match state.go modes or be empty), no `id` uniqueness check, and no editor support (autocomplete, inline validation). The file has a Markdown header line that the parser strips with a crude `strings.Index` hack — there is no canonical schema definition.

### 3. Soft ADR citation gates

At plan-approve, citing a Superseded ADR produces a warning — not a blocking error. A plan can be approved while relying on dead architectural decisions. Similarly, citing a Proposed (unaccepted) ADR is advisory only. At impl-approve, there is no ADR re-check at all — an ADR could be superseded after plan approval and before implementation closes, with no detection.

### 4. Missing ADR consideration gate

The spec-approve gate validates structural completeness (sections, acceptance criteria, open questions) but does not prompt for or validate ADR impact assessment. The `## ADR Touchpoints` section is required by the spec template but not validated for substance — it can contain a placeholder or "None" and still pass. There is no gate ensuring the question "does this spec require new ADRs or changes to existing ones?" is explicitly answered.

### 5. Missing domain lifecycle gate

The policy `domain-operations-require-approval` states that "Adding, splitting, or merging domains requires explicit human approval and must produce an ADR." But this is never enforced. The spec validator checks that `## Impacted Domains` exists as a section but does not validate its content against known domains. A spec can name a domain that doesn't exist (implying domain creation) or restructure domain boundaries without any gate triggering. There is no prompt asking "does this spec require adding, splitting, or merging domains?" and no validation that new domain names are intentional.

## Impacted Domains

- **workflow**: Mode transition gates, validation rules, formula step enforcement.
- **context-system**: Policy parsing, ADR filtering, context-pack assembly (consumers of governance artifacts).
- **core**: Doctor checks, instruct template guidance updates.

## ADR Touchpoints

- [ADR-0001](../../adr/ADR-0001.md): DDD governance and ADR lifecycle — this spec strengthens enforcement of ADR-0001's governance invariants.
- [ADR-0005](../../adr/ADR-0005.md): Explicit state — policy validation state and sync metadata follow the same persistence conventions.
- [ADR-0013](../../adr/ADR-0013.md): Formula-based lifecycle — this spec adds validation steps within existing formula gates, not new formula steps.

## Requirements

### Policy sync and schema

1. `policies.yml` must conform to a JSON Schema definition shipped with the MindSpec binary (embedded asset).
2. The schema must enforce: required fields (`id`, `description`, `severity`), `severity` enum (`error`, `warning`), `mode` enum (valid state.go modes or empty), and `id` uniqueness.
3. `policies.yml` must be pure YAML with no Markdown header. The parser hack that strips content before `policies:` must be removed.
4. `mindspec validate policies` must validate the file against the schema and check that every `reference:` path resolves to an existing file.
5. `mindspec validate policies` must warn when a policy's `reference:` points to a file containing a superseded ADR.
6. `mindspec doctor` must include a policy-health check covering schema conformance, reference resolution, and staleness detection.
7. Plan-mode instruct guidance must remind the agent to update `policies.yml` when introducing governance-affecting ADRs.
8. The doc-sync validator (`ValidateDocs`) must treat `policies.yml` as a sync target — if ADR files change, a missing policy file change should produce a warning.

### ADR citation hardening

9. At plan-approve, citing a Superseded ADR must be a blocking error, not a warning. The plan must cite the superseding ADR or justify the citation explicitly.
10. At plan-approve, citing a Proposed ADR must be a blocking error. The ADR must be Accepted before the plan can be approved.
11. At impl-approve, all ADRs cited in the approved plan must be re-checked. If any have been superseded since plan approval, impl-approve must block with an actionable diagnostic.
12. The `## ADR Fitness` section in plans must be validated for substance — it must contain at least one row per cited ADR with a verdict (Conform/Diverge/Not Applicable).

### ADR consideration gate at spec-approve

13. Spec validation must check that `## ADR Touchpoints` contains at least one substantive entry (not placeholder text like "None", "N/A", "TBD").
14. If `## ADR Touchpoints` lists zero touchpoints, spec validation must require an explicit entry stating "No ADR impact — rationale: <explanation>" to pass.
15. Instruct guidance for spec mode must include the prompt: "Does this spec require new ADRs or changes to existing ones? If yes, draft them now. If no, document why in ADR Touchpoints."

### Domain lifecycle gate at spec-approve

16. Spec validation must parse the `## Impacted Domains` section and compare listed domain names against known domains (domains with existing `docs/domains/<name>/` directories, or post-034 canonical equivalents).
17. If a spec lists a domain name that does not match any known domain, spec-approve must block with an explicit gate: "Domain '<name>' does not exist. Is this a new domain? Domain creation requires human approval and an ADR."
18. If a spec's impacted domains suggest a split or merge (e.g., functionality moving between domains, or a new domain absorbing responsibilities from an existing one), the instruct guidance must prompt: "Does this spec require adding, splitting, or merging domains? If yes, draft an ADR. If no, confirm domain boundaries are unchanged."
19. Instruct guidance for spec mode must include the domain lifecycle consideration prompt alongside the ADR consideration prompt.
20. `mindspec doctor` must validate that all domains referenced in open specs have corresponding domain directories.

## Scope

### In Scope

- `policies.schema.json` (new, embedded) — JSON Schema for `policies.yml` structure validation.
- `internal/contextpack/policy.go` — remove Markdown header strip hack; validate against schema.
- `internal/validate/plan.go` — promote ADR citation warnings to errors; add ADR Fitness substance check.
- `internal/validate/spec.go` — add ADR Touchpoints substance validation; add domain lifecycle gate (compare listed domains against known domain directories).
- `internal/validate/policies.go` (new) — policy schema validation, reference resolution, and staleness checks.
- `internal/validate/docsync.go` — extend to track `policies.yml` as a sync target when ADR files change.
- `internal/approve/impl.go` — add ADR re-check at impl-approve.
- `internal/doctor/` — add policy-health and domain-reference checks.
- `internal/instruct/templates/spec.md` — add ADR consideration and domain lifecycle prompts.
- `internal/instruct/templates/plan.md` — add policy-update reminder.
- `cmd/mindspec/validate.go` — wire `validate policies` subcommand.

### Out of Scope

- Automated policy generation from ADR content (policies remain human-authored).
- Policy enforcement at runtime (policies are context-pack inputs, not runtime guards).
- ADR approval automation (`mindspec adr accept` command — that's a separate concern).
- Changes to the Beads formula structure (this spec adds validation within existing gates, not new formula steps).
- Automated domain split/merge detection from code analysis (detection is prompt-driven and human-gated, not heuristic).

## Non-Goals

- Making every governance check a hard blocker with zero override. Some checks (like doc-sync policy warnings) remain advisory — only ADR citation, ADR Touchpoints, and domain lifecycle checks become hard gates.
- Removing human judgment from ADR fitness evaluation or domain boundary decisions. The substance checks validate that the evaluation happened, not that the verdict is correct.
- Retroactively validating all existing specs/plans against the new rules. New rules apply going forward.
- Automatically inferring domain splits/merges from code structure. Domain lifecycle decisions are human-driven; the gate ensures the question is asked, not answered.

## Acceptance Criteria

- [ ] `policies.yml` conforms to a shipped JSON Schema; `mindspec validate policies` validates structure, field enums, and id uniqueness.
- [ ] `policies.yml` is pure YAML with no Markdown header; parser hack is removed.
- [ ] `mindspec validate policies` reports broken `reference:` paths and superseded-ADR references.
- [ ] `mindspec doctor` includes policy-health check results (schema, references, staleness).
- [ ] Plan-approve blocks when a cited ADR is Superseded (error, not warning).
- [ ] Plan-approve blocks when a cited ADR is Proposed/unaccepted (error, not warning).
- [ ] Plan-approve blocks when `## ADR Fitness` section is missing or has no per-ADR verdicts.
- [ ] Impl-approve re-checks all plan-cited ADRs and blocks if any were superseded post-approval.
- [ ] Spec-approve blocks when `## ADR Touchpoints` contains only placeholder text.
- [ ] Spec-approve blocks when `## Impacted Domains` lists a domain with no corresponding domain directory.
- [ ] Spec-mode instruct guidance includes explicit ADR consideration and domain lifecycle prompts.
- [ ] Plan-mode instruct guidance includes policy-update reminder for governance-affecting ADRs.
- [ ] Doc-sync validation warns when ADR files change but `policies.yml` is untouched.
- [ ] `mindspec doctor` validates that domains referenced in open specs have corresponding directories.

## Validation Proofs

- `mindspec validate policies` with a broken reference path → reports error with file path and suggested fix.
- `mindspec validate policies` with invalid `severity: critical` → reports schema violation.
- `mindspec validate policies` with duplicate policy `id` → reports uniqueness violation.
- `mindspec approve plan <id>` with a Superseded ADR citation → blocks with "ADR-NNNN is superseded by ADR-MMMM; update citation" message.
- `mindspec approve plan <id>` with missing ADR Fitness section → blocks with "## ADR Fitness section required" message.
- `mindspec approve impl <id>` after an ADR cited in the plan is superseded → blocks with re-check diagnostic.
- `mindspec approve spec <id>` with `## ADR Touchpoints` containing "None" → blocks with guidance to provide rationale.
- `mindspec approve spec <id>` with `## Impacted Domains` listing "payments" when no `docs/domains/payments/` exists → blocks with "Domain 'payments' does not exist. Is this a new domain?" gate.
- `mindspec doctor` on a repo with stale policy references → reports policy-health warning.
- `mindspec doctor` on a repo where a spec references a non-existent domain → reports domain-reference warning.

## Open Questions

- [ ] Should `validate policies` also check that every accepted ADR with governance implications has a corresponding policy entry? This would be a stronger sync guarantee but requires defining "governance implications" heuristically.
- [ ] Should the ADR re-check at impl-approve also verify that no *new* ADRs were accepted in the relevant domains since plan approval? This would catch cases where a new ADR introduces constraints the plan didn't account for.
- [ ] Should the domain lifecycle gate at spec-approve also detect domain *removal* (spec removes a domain from impacted list that previous specs relied on)? This is a weaker signal but could catch accidental boundary erosion.

## Approval

- **Status**: DRAFT
- **Approved By**: —
- **Approval Date**: —
- **Notes**: —
