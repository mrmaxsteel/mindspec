# ZFC-Aware Design Panel — the mindspec-side fix for `mindspec-6ou2`

**Repo**: `/Users/Max/replit/mindspec` (the mindspec source). This is a **DESIGN review of a fix DIRECTION — there is no code change yet.** You are choosing/critiquing the right mindspec-side fix for bead **`mindspec-6ou2`**, with **Zero Framework Cognition (ZFC, ADR-0036)** as the primary lens. Read-only: read source, ADRs, specs, `bd show mindspec-6ou2`. Do NOT edit anything; do NOT `gh pr checkout` (shared checkout).

## The bug (`mindspec-6ou2`, P2 open)
`mindspec plan approve`'s ADR-semantic gates fail for a spec whose `## Impacted Domains` and cited ADRs reference the SAME underlying paths, because the two sides are compared in **different namespaces**:
- **Spec side** — `normalizeImpactedDomains` (`internal/validate/plan.go:~141`, via `internal/validate/ownership_resolve.go`) resolves each path-like Impacted-Domains entry through `OWNERSHIP.yaml` globs to a single **domain NAME** (e.g. `spec056`).
- **ADR side** — `checkADRCitations` (`plan.go:~483`, `intersectFold`) and `checkADRCoverage` / `coverageOf` (`plan.go:~660`, `domainSliceContains`) compare that resolved name against the cited ADR's raw `Domain(s)` line **literals** — which hold path/tuple strings (`api/app/lola/`, `api (lola, tools)`). `spec056` ∈ {raw literals} is always false. No transitive path-overlap resolution → `[impacted-domains-resolve]`, `[adr-cite-irrelevant]`, `[adr-coverage-missing]`.

Discovered implementing **lola spec 056** (canonical-layout project). It is **layout-agnostic** — confirmed by a prior reviewer: the only layout-aware calls (`workspace.DomainsDir`, `adrStoreForSpec`) return identical names/content flat vs canonical. It was **exposed by spec 100** (shipped in v0.10.0), which made path-like Impacted Domains resolve-by-glob on the spec side but did NOT change the ADR side.

### Real-world workarounds the bug forces (all bad)
1. A **catch-all `OWNERSHIP.yaml`** (`name: spec056`) claiming every path the spec touches (`api/app/lola/**`, `api/tests/**`, `api/alembic/**`, `infra/tofu/**`, …) so they resolve to one domain. A prior reviewer found this monopolizes shared trees (a latent **multi-owner hard-error trap** for the next spec — `normalizeImpactedDomains` errors when a path resolves to >1 domain), works today only because the *other* domain's globs are dead (trailing-slash, no `/**`), AND under-claims the spec's own changed files.
2. Manually appending `, spec056` to every cited ADR's `Domain(s)` line (ADR churn, easy to forget).
3. Reformatting Impacted Domains to avoid backtick-wrapped paths.

### The bead's three proposed fixes (evaluate these + propose better)
- **(a)** Resolve BOTH sides through OWNERSHIP and intersect by resolved name OR raw **path-overlap** (transitive: A and B intersect if any claimed path of A overlaps any of B).
- **(b)** Add a `:: spec` umbrella keyword to OWNERSHIP that the validator treats as "claims paths under the umbrella of spec X" so overlapping cited ADRs satisfy the check.
- **(c)** Loosen the check from error → warning until OWNERSHIP coverage is hand-curated.
- A prior reviewer recommended: **adopt (a), reject (b) (bakes in the domain-as-spec anti-pattern), (c) stopgap only**, AND move lola to durable disjoint short-tag domains.

## THE GOVERNANCE TENSION (read both, central to your verdict)
- **ADR-0032** (`Semantic ADR Coverage Gates`, Accepted) sub-decision 1: *"Domain identifier is the OWNERSHIP.yaml directory name. All three artifacts (spec `## Impacted Domains`, OWNERSHIP location, ADR `Domains` field) MUST use the same short-tag identifier set (e.g. `core`, `execution`). Comparison is case-folded, exact set intersection. No aliases or hierarchy in v1. **Rejected alternatives: path-like identifiers (ambiguous — `internal/foo` vs `foo`)**; free-form tags."* → Fix (a)'s path-overlap **reverses an Accepted ADR decision**. lola's path-like usage is arguably a **contract violation** of ADR-0032, not a use to support.
- **ADR-0036** (`Ownership Discovery — Zero Framework Cognition`, Accepted) is the ZFC anchor: *"heuristic classification is forbidden — semantic decisions (which paths a domain owns) belong to a coding agent or human operator who inspected the repo, never to framework guesswork. The framework only writes the empty stub + a populate prompt; population is cognitive work routed to the agent."* The framework does **deterministic mechanical checks on DECLARED structured data**, never free-form heuristic parsing.

## ZFC-aware questions every reviewer must answer
1. **Does fixing 6ou2 require adding framework cognition?** Specifically: fix (a) "resolve the ADR side through OWNERSHIP" — does it force the framework to **heuristically parse free-form ADR `Domain(s)` strings** (`api (lola, tools)`) to glob-resolve them? If so that is itself a **ZFC violation** (the exact free-form-string heuristic ADR-0036 forbids). Is path-overlap on declared globs ZFC-clean, or does it smuggle in ambiguity ADR-0032 rejected?
2. **Enforce vs accommodate.** Is the ZFC-clean fix to **REJECT path-like / tuple identifiers and TEACH the short-tag contract at point-of-use** (deterministic check on declared data + guidance), rather than build path-heuristics or a `:: spec` keyword to accommodate the violation? What should the validator's error message *teach*?
3. **Did spec 100 point the wrong way?** spec 100 made the spec side resolve path→name (a partial accommodation of path-like identifiers). Is the right north star spec-100's direction (support paths) or ADR-0032's (enforce short-tags)? Should spec 100's resolution itself be reconsidered?
4. **Governance**: which fix needs an ADR amendment/supersede (ADR-0032/0036)? Is this a bug-fix or a spec? Migration impact on repos already using short-tags correctly (mindspec's own ADRs) vs path-like (lola).
5. **Smallest correct + ZFC-aligned fix** — name it. Could be one of a/b/c, a variant, or "enforce + teach."

## Code map (read these)
`internal/validate/plan.go` (`normalizeImpactedDomains`, `checkADRCitations`, `intersectFold`, `checkADRCoverage`, `coverageOf`, `domainSliceContains`), `internal/validate/ownership_resolve.go` (`normalizeImpactedDomains` resolution + the >1-owner error ~147-155), `internal/validate/ownership.go` (`LoadOwnership`, `GlobMatch` ~343), `internal/validate/divergence.go` (`adr-divergence-unowned` ~211). ADRs: `ADR-0032`, `ADR-0036`, `ADR-0031` (doc-sync gate), `ADR-0017`. Spec 100 (`.mindspec/specs/100-ownership-gate-resolution/spec.md`) for what shipped.

## Verdict
For the mindspec-side fix direction: **state your recommended fix** (a / b / c / variant / "enforce+teach"), its **ZFC-alignment** (compliant / smuggles cognition / neutral), the **ADR move** required, and what to **avoid**. Output JSON to `/Users/Max/replit/mindspec/review/6ou2-fix-zfc/<your-slot>-round-1.json`:
`reviewer_id`, `recommended_fix` (short string), `zfc_verdict` (COMPLIANT / SMUGGLES_COGNITION / NEUTRAL — for the fix you recommend), `confidence` (0-1), `rationale` (≤220 words), `adr_action` (what ADR amend/supersede/none is needed), `findings` (array of {severity, area, issue}), `fixes_assessed` (object mapping a/b/c → keep/reject/stopgap + one-line why).
Do NOT run `go test ./internal/harness/...`.
