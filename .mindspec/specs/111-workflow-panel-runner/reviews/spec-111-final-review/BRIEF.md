# spec-111-final-review — Final Review Panel (12 reviewers, four families)

**Worktree**: `/Users/Max/replit/mindspec/.worktrees/worktree-spec-111-workflow-panel-runner`
**Branch**: `spec/111-workflow-panel-runner` @ **4e8a1f452f7835beb42b01c500695cbb1ea5a649**
**Panel**: 12 slots — F1–F3 Fable, O1–O3 Opus, S1–S3 Sonnet, **G1–G3 codex/GPT-5.5 (REAL codex — it is back)**. **Pass = ≥11 APPROVE, no REJECT.**
**This is the final gate before `mindspec impl approve` merges spec 111 → main.**

**READ-ONLY RULE (MANDATORY)**: verdict JSON only; pin reads to `4e8a1f45`; leave `git status` clean. **Any scratch repo/config/binary MUST use ABSOLUTE `/tmp` paths (or `t.TempDir()`) — NEVER a relative `.mindspec/…` write** (a relative scratch write + this harness's cwd-reset corrupted a sibling worktree earlier this run).

## The spec's changeset (review THIS)
spec/111 already CONTAINS main (109/110/112 were merged into it), so its changeset vs main is exactly 111's own contribution:
```
git -C <worktree> diff origin/main...4e8a1f45                              # 111's changeset (~106 files incl. review artifacts)
git -C <worktree> diff origin/main...4e8a1f45 -- ':!*/reviews/*' ':!*/review/*'   # code+docs only
```
**Because spec/111 contains main, the eventual impl-approve is a clean spec-ahead-of-main merge (no conflict resolve).** And because 110's `mindspec panel` verbs are now in the base, you CAN build the branch binary and run the `/ms-panel` workflow's registration path (`mindspec panel create/verify/tally`) end-to-end.

## What spec 111 delivers (read `spec.md` in full — Goal + R1–R9)
The first **orchestration runner adapter**: a Claude Code dynamic workflow `/ms-panel` (`.claude/workflows/ms-panel.js`, embedded + installed to the Claude target) that runs a panel behind spec 110's agent-neutral CLI verbs + verdict schema, selected by spec 109's `runner:` key.
- **The workflow** (`ms-panel.js`): given `{slug, spec, target, bead_id?, round, sha?, lenses[], mix, claude_sub_on_quota?}`, it registers via `mindspec panel create`, fans out reviewers per config `panel:` mix (claude slots = `agent()` steps; codex slots = wrapper agents that exec codex read-only-sandboxed and write the verdict themselves — codex never writes files), substitutes a `claude-sub` on a quota wall per `panel.substitution.claude_sub_on_quota`, and returns `panel verify` + `panel tally` output verbatim. **The script does NO shell/file I/O itself** (Claude Code workflow limit — every CLI touch + file write is an `agent()` step).
- **`ms-panel-run`** shrinks to lens composition + one workflow invocation when `runner: claude-code-workflow`, retaining the full manual launch path when `runner: claude-code-skills` (the DEFAULT until the workflow path is proven). **`ms-panel-tally`** gains a workflow-result note.
- **Ownership** (Bead 1): `.claude/workflows/**` claimed for the workflow domain.

**Load-bearing invariants:** the workflow is an ADAPTER — it writes the SAME artifacts 110 defines, is **never a second decision authority**, and **never runs `mindspec complete`** (that string must appear nowhere in `ms-panel.js`). `PanelGateDecision`/`mindspec complete` are unchanged. `internal/panel` stays a config-free leaf.

Delivered across 3 merged beads (each bead-panel-passed): 9cyu.1 (ownership 8/8), 9cyu.2 (the workflow adapter — 5A/3RC → fixed a real RCE-class shell-injection + a hollow exact-set scanner → 8/8), 9cyu.3 (runner dispatch + tally note — 5A/3RC → fixed a spec-mandated `claude_sub_on_quota` feature-regression → 8/8).

## The security surface (F3 Fable + G1 codex — probe HARDEST, empirically)
The `ms-panel.js` command chokepoint was hardened during bead review; CONFIRM it holds on the spec branch:
- **`ALLOWED_CLI`** is a static literal 4-command array; `buildCommand` is the sole assembler (fixed flags from per-verb templates, values-only leading-dash guard); `mindspec complete` appears nowhere.
- **Input hardening**: `validateArgs` runs `validatePathElement` (rejects empty/`.`/`..`/`/`/`\`/control-bytes/**whitespace**) AND `validateShellSafe` (rejects `;`/`|`/`&`/`` ` ``/`$(`/newline) on slug/spec/bead_id; `target` gets the branch-name-safe grammar + metachars; `validateMix` enum-checks `family ∈ {claude,codex}` at entry; `round` a positive integer. **Re-run the RCE attack**: build the binary, drive `validateArgs` (extract the pre-`agent()` functions to a /tmp node scratch) with `slug`/`spec` = `x$(id)`, `x;id`, `x|sh`, `` x`whoami` ``, `foo bar` — all MUST throw. Any bypass = REJECT-worthy.
- **The exact-set test** (`TestMsPanelWorkflow_AllowedCLIExactSet`) genuinely catches an injected literal (the `SHELL_METACHAR_RE` backtick is `\x60`-escaped so `scanJSLiterals` doesn't desync).
- **The anti-laundering ladder (R3b)**: a rendered-but-malformed verdict fails CLOSED to MISSING (never substituted); substitution (R4) only fires on a quota wall with NO verdict rendered.

## Per-family lens assignments (12 distinct angles)
### Fable (adversarial)
- **F1 — spec-goal fidelity**: both outcomes delivered + every R1–R9 falsification clause satisfied? Any AC without implementation/test?
- **F2 — cross-bead coherence**: ownership (9cyu.1) → workflow (9cyu.2) → skills dispatch (9cyu.3) compose with no half-wiring; the runner-dispatch arg tuple matches the workflow's args contract (incl. `claude_sub_on_quota`); the interfaces.md/runbook doc-sync is consistent across beads.
- **F3 — security + adapter invariants**: the injection surface (above) holds; the workflow is never a 2nd decision authority, never runs `mindspec complete`; `internal/panel` config-free leaf.

### Opus (completeness / architecture)
- **O1 — requirement completeness**: walk R1–R9; each needs implementation AND a test pinning its falsification clause.
- **O2 — ADR compliance**: ADR-0040 (portability — the workflow is an ADAPTER at the artifact+CLI contract, degraded modes: no-workflow→skills path), ADR-0037 (single decision home — `PanelGateDecision` unchanged, workflow adds no interpreter), ADR-0035 (recovery lines), ADR-0036 (`.claude/workflows/**` ownership).
- **O3 — scope / honest boundaries**: no `PanelGateDecision`/`mindspec complete` change; workflow does no consolidation/decision; `runner:` default stays `claude-code-skills`; `loop:`/`models:` not enforced.

### Sonnet (empirical / test / regression)
- **S1 — end-to-end**: build the branch binary (abs /tmp) — it now HAS 110's panel verbs. Run the runner dispatch (`mindspec config show` → `runner:`); exercise the workflow's registration path (`mindspec panel create` with the args the workflow builds); confirm the install (`mindspec setup` installs `ms-panel.js` to the Claude target only). If feasible, drive the plan's Manual e2e shim scenarios (now runnable).
- **S2 — test quality**: the workflow structural tests (embed==copy; exact-set; positive-enumeration) + the ownership + install tests — falsifiable, not decorative? Spot-check by reverting.
- **S3 — regression**: full `go test ./...`; no regression; the 2 KNOWN pre-existing failures (`internal/harness` timeout, `internal/instruct` `TestRun_IdleNoBeads` z4ps) reproduce at the merge-base and are NOT 111's.

### codex/GPT-5.5 (REAL codex — empirical injection / schema / integration)
- **G1 — injection empirical**: re-attack `ms-panel.js` hardest (the RCE surface above) with the built binary + a /tmp node scratch driving `validateArgs`/`validateMix`/`buildCommand`; confirm every payload throws, the exact-set catches an injected literal, the ladder fails closed. This is the load-bearing security re-check.
- **G2 — schema/type + args contract**: the workflow args tuple + verdict schema (110 shape) + the fail-closed semantics (`claude_sub_on_quota` absent → false); the codex-slot wrapper (read-only sandbox, `.codex.log`, one-object-stdout rule); no type/parse hole.
- **G3 — integration end-to-end**: does 111 consume 110's verbs + 109/112's config cleanly? Build the binary; run `mindspec panel create/verify/tally` the way the workflow does; confirm the runner-dispatch → `/ms-panel` → verify/tally-return chain is coherent; confirm the install wiring (RunClaude only).

## Your job
Evaluate the whole spec end-to-end against Goal + R1–R9. APPROVE means: both outcomes delivered, all requirements implemented + tested to their falsification clauses, the adapter invariants hold (never a 2nd decision authority, never `mindspec complete`, config-free leaf), the injection surface is genuinely closed, ADRs honored, the runner-dispatch + install are correct, no regressions, and it merges cleanly (it already contains main).

Verdict: APPROVE / REQUEST_CHANGES / REJECT. Output JSON to `<this-dir>/<your-slot>-round-1.json` with keys: `reviewer_id` ("<slot> <family>", e.g. "F1 fable" / "G1 gpt-5.5"), `verdict`, `confidence` (0–1), `rationale` (≤200 words), `concrete_changes_required` (empty if APPROVE), `findings`. An artifact-gate finding may set `"hard_block": true`.
