# MindSpec Agent Instructions

This repository uses **MindSpec** for spec-driven development. All agents must follow the mode system, Beads conventions, and governance rules defined here.

## Mode System

All work follows a three-phase approach:

### Spec Mode (Default)
- **Permitted**: Markdown files only (specs, domain docs, glossary, ADR drafts)
- **Focus**: User value, acceptance criteria, impacted domains, ADR touchpoints, open questions
- **Exit**: Explicit user approval via `/spec-approve`

### Plan Mode
- **Artifact**: `docs/specs/<id>/plan.md` — created from [`docs/templates/plan.md`](docs/templates/plan.md) at Plan Mode start (`status: Draft`), iteratively edited, approved via frontmatter state change (`status: Approved` + `approved_at`, `approved_by`, `approved_sha`, `bead_ids`, `adr_citations`)
- **Permitted**: `plan.md`, Beads entries (implementation beads), ADR proposals
- **Focus**: Bounded work chunks with verification steps, ADR review, dependency mapping
- **Required**: Review domain docs + accepted ADRs + Context Map before planning
- **Exit**: Explicit user approval via `/plan-approve`

### Implementation Mode
- **Permitted**: Code, tests, configuration, documentation updates
- **Requires**: Approved spec + approved plan + assigned bead + worktree
- **Obligations**: Scope discipline, doc-sync, proof-of-done, ADR compliance, worktree isolation

> **Rule**: Never create or modify code without an approved spec AND an approved plan.

---

## Beads Integration

Beads is the **execution tracking substrate** (not a planning system):

- Spec beads: concise summary + link to canonical spec file. Do not embed long-form specs.
- Implementation beads: scope, micro-plan, verification steps, dependencies.
- Keep the active workset small. Rely on git history + docs for archival traceability.
- Beads entries must remain concise and execution-oriented.

See [ADR-0002](docs/adr/ADR-0002.md) for full Beads integration strategy.

---

## Git Workflow

### Clean Tree Rule

A clean working tree (`git status` shows no changes) is a **hard precondition** for:

- Starting new work (picking up a bead via `mindspec next` or `mindspec pickup`)
- Switching modes (Spec → Plan → Implement → Done)

If the tree is dirty: **commit or revert**. Never auto-stash.

### Milestone Commits

Mode transitions produce explicit commits:

- **Spec → Plan**: `spec(<bead-id>): <summary>` — spec artifact + bead update
- **Plan → Implement**: `plan(<bead-id>): <summary>` — plan artifacts + spawned beads
- **Implement → Done**: `impl(<bead-id>): <summary>` — code, tests, docs, bead closure

Normal commits during a mode are fine. Use `chore(<bead-id>): ...` for cleanup/tooling.

Always co-commit `.beads/` changes alongside the relevant work.

### Preflight (before any forward-progress work)

1. Confirm correct worktree/branch for the active bead
2. Confirm working tree is clean — if not, commit or revert first
3. Confirm active bead exists and is in the expected state
4. Only then proceed

If the tree is dirty when you begin, switch to **recovery mode**: provide commit/revert guidance rather than forward-progress instructions.

## Worktree Execution

All implementation work runs in **isolated git worktrees**:

- Worktree naming includes the bead ID
- Changes are isolated per bead
- Closing a bead requires evidence + doc updates + clean state sync

---

## ADR Governance

If implementation or planning requires changes that diverge from an accepted ADR:

1. **Stop** immediately
2. Inform the user: specify the ADR and the nature of divergence
3. Present options: continue-as-is vs. propose new superseding ADR
4. If user approves divergence: create a new ADR superseding the old one
5. The new ADR must be accepted before work resumes

> **Rule**: ADR divergence always triggers a human gate. The ADR is the decision artifact.

---

## Domain Awareness

MindSpec uses DDD-inspired domains as first-class primitives:

- Specs must declare impacted domains
- Context Packs route content based on domain boundaries
- Domain operations (add/split/merge) require human approval and produce ADRs

See [ADR-0001](docs/adr/ADR-0001.md) for DDD enablement details.

---

## Required Workflows

| Command | Purpose |
|:--------|:--------|
| `/spec-init` | Initialize a new specification |
| `/spec-approve` | Request Spec → Plan transition |
| `/plan-approve` | Request Plan → Implementation transition |
| `/spec-status` | Check current mode and active spec/bead state |

---

## Documentation Sync

Every code change must:
- Update corresponding documentation
- Keep acceptance criteria aligned
- Add glossary entries for new concepts

**"Done" includes doc-sync.**

---

## Key Documentation

| Document | Purpose |
|:---------|:--------|
| [CLAUDE.md](CLAUDE.md) | Claude Code project instructions |
| [mindspec.md](mindspec.md) | Product specification (source of truth) |
| [MODES.md](docs/core/MODES.md) | Mode definitions and transitions |
| [ARCHITECTURE.md](docs/core/ARCHITECTURE.md) | System design and invariants |
| [CONVENTIONS.md](docs/core/CONVENTIONS.md) | File organization and naming |
| [GLOSSARY.md](GLOSSARY.md) | Term definitions for context injection |
| [policies.yml](architecture/policies.yml) | Machine-checkable policies |
| [ADR-0001](docs/adr/ADR-0001.md) | DDD enablement + context packs |
| [ADR-0002](docs/adr/ADR-0002.md) | Beads integration strategy |

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
