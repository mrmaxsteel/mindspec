# v0.10.0 — Flattened `.mindspec/` layout

## Headline

`.mindspec/` is **flattened**: `specs/`, `adr/`, `domains/`, `core/`, and
`context-map.md` are now top-level children of `.mindspec/` — the
`.mindspec/docs/` wrapper is gone. Panel reviews are **co-located** under
`<spec-dir>/reviews/`. Repo dogfood documentation moved to a top-level
`project-docs/` tree. The vestigial `glossary.md` and `policies.yml` were
dropped.

## Non-breaking & opt-in

**Existing projects keep working with NO action required.** A multi-tier
resolver (flat → canonical → legacy) reads every layout, first-exists-wins, so
pre-flatten checkouts resolve exactly as before. Writes stay in your project's
current layout until you explicitly opt in — **upgrading the binary alone
changes nothing on disk.**

## Migrating an existing project (optional)

```bash
mindspec migrate layout
```

A transactional mover does **two commits per move** (a pure `git mv`, then a
link-rewrite), so history stays clean and bisectable.

**Preconditions:** a clean working tree and no unmerged pre-flatten branch. If an
unrelated stale pre-flatten branch trips the precondition, exempt it with
`--allow-branch <name>` (repeatable) or bypass the scan with `--force` (logged).

`mindspec migrate layout --abort` rolls back a **pre-publish** run; once the run
is published the flatten is **forward-only**. Run `mindspec doctor` afterward to
confirm links resolve.

## Also in this release

- **Directional cross-layout merge guard** — hard-fails the regression
  direction so a flattened branch can't be silently un-flattened.
- **Layout-aware panel gate** — the `complete` gate scans the honored review
  location(s) for the tree's detected layout.
- **`doctor` layout detection** — reports the detected docs layout and flags a
  tree that would flatten on the next migrate.
- **`migrate layout` hardening** — precondition scoping and a wider link-check.

## Governance

- **ADR-0039** (Flat `.mindspec/` Layout v2) — **Accepted**.
- **DOCS-LAYOUT.md** and **ADR-0037** amended (reviews co-location).

## Known issues

- On a **branch-protected** `main`, `mindspec impl approve` can momentarily leave
  the bead tracker's committed `.beads/issues.jsonl` out of sync with the
  source-of-truth Dolt store, because the lifecycle's finalize commit cannot land
  directly on protected `main`. A one-time manual `.beads` sync resolves it (the
  post-merge hook is then idempotent). Tracked as `mindspec-wu7t`. Normal feature
  work and non-protected repositories are unaffected.
