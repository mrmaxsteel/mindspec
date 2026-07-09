# spec-111-bead2 — consolidated round-1 changes

Tally: 5 APPROVE (R1 0.9, R2 0.97, R3 0.95, R4 0.93, R6 0.9) / 3 REQUEST_CHANGES (R5 sonnet 0.85, R7 fable 0.8, R8 sonnet-sub 0.9) / 0 REJECT. Threshold 7/8 not met → fix round. The ALLOWED_CLI static array, buildCommand chokepoint structure, mirror byte-identity, embed + Claude-target-only install, created-count (base+1), and 110-contract match (R6 cross-checked against 110's actual merged CLI on main) are all verified. **Three reviewers independently found a real RCE-class injection hole**, and R8 found the guard-test that should catch it is hollow.

## Fix 1 — Shell command injection via slug/spec (R5 + R7 + R8 CONVERGENT, load-bearing, RCE-class)
`buildCommand` assembles a CONCATENATED shell command string (ms-panel.js:78 `` `${verb} ${slug} --spec ${specId} --target ${target}` ``). `validateArgs` (~204-205) guards `slug` and `spec` with `validatePathElement` ONLY (rejects empty/`.`/`..`/`/`/`\`/control-bytes) — NOT `validateShellSafe`. But `target` and `bead_id` DO get the shell-metachar guard, precisely because (per the file's own comment) "it flows into a built command line" — the identical reasoning applies to slug/spec, an internal inconsistency. The leading-dash guard only checks `value.startsWith("-")`, so it misses internal tokens. Empirically verified (node, live): `slug="x$(rm -rf ~)"`, `x;id`, `x|sh`, `` x`whoami` ``, `foo --target evil` ALL pass and reach the built command, which the workflow hands to an agent → Bash → shell substitution/chaining/flag-smuggling EXECUTES.
- **Fix**: in `validateArgs`, add `validateShellSafe("slug", a.slug)` and `validateShellSafe("spec", a.spec)`, mirroring the guard `target`/`bead_id` already receive.
- **Also (R5+R7)**: reject WHITESPACE in `slug`/`spec`/`bead_id` (as `validateTarget` does at line ~163) — a space-containing value word-splits the shell string into extra argv tokens even without a metacharacter.

## Fix 2 — validateMix does not enum-check family at entry (R5)
`validateMix` (~181-193) validates `entry.family` with `validatePathElement` only. The `{claude, codex}` enum check lives in `runSlot` (Step 3, ~390-398), AFTER Step 2's `panel create` has already run and mutated state — violating the stated "any failure aborts before Step 2" invariant for this field.
- **Fix**: in `validateMix`, reject any `entry.family` not in the closed enum `{"claude", "codex"}` so an unrecognized family aborts inside `validateArgs`, before Step 2.

## Fix 3 — Hollow positive-enumeration guard (R8, test-integrity)
`TestMsPanelWorkflow_AllowedCLIExactSet`'s positive-enumeration is supposed to catch any stray `mindspec`/`codex`-bearing literal outside the array (the plan's "strengthens AC4 beyond present/absent grep"). But `SHELL_METACHAR_RE = /[`;|&\n]|\$\(/;` (ms-panel.js:127) contains a literal BACKTICK inside the regex char class, and the Go test's `scanJSLiterals` (workflow_test.go:50-93) is a naive quote scanner with no regex-literal awareness — it mistakes that backtick for a template-literal start and DESYNCS its string-boundary tracking from byte ~5836 onward. R8 injected a real (non-comment) `mindspec panel create` literal at ms-panel.js:282 and the exact-set test STILL PASSED — the guard is hollow past the desync point.
- **Fix (R8's confirmed one-liner)**: escape the backtick in `SHELL_METACHAR_RE` as `\x60` (functionally identical match, verified in node) so the scanner does not desync. After this, the injected-literal mutation is correctly caught. (Optionally also harden `scanJSLiterals` to skip regex literals, but the `\x60` escape is the minimal, confirmed fix.)

## Non-blocking (do NOT fix in this round)
- R7 (follow-up bead filed): `SHELL_METACHAR_RE` matches `$(` but NOT a bare `$`, so `$HOME`-style variable expansion survives on slug/spec/target/bead_id if the string is shell-executed — a residual expansion vector across ALL fields. Out of scope for this minimal fix.
- R6 (optional, minor): the registration prompt says "find the directory path it reports" in prose rather than citing the literal `panel directory:` prefix — low risk given the schema-constrained return; a prose tweak.
- R6 (note for e2e): confirm `panel tally` stdout is captured regardless of its non-zero Block exit — validate once this branch is past 110 at impl-approve.

## Constraints for the fix author
- ONE commit on `bead/mindspec-9cyu.2`: `fix(111): shell-safe slug/spec + whitespace guard, validateMix family enum, un-hollow the exact-set regex-scan [mindspec-9cyu.2]`.
- Edit `plugins/mindspec/workflows/ms-panel.js` AND its byte-identical mirror `.claude/workflows/ms-panel.js` IDENTICALLY; `internal/validate`... no — only the workflow files + `plugins/mindspec/workflow_test.go` if the `\x60` change or a new validator test warrants it. Keep both ms-panel.js copies `diff -q` clean.
- All existing tests stay green; the exact-set test must now CATCH an injected literal past the old desync point. `go build ./...`; `go test -count=1 ./plugins/... ./internal/setup/...`.
- Scratch under ABSOLUTE /tmp only. No push/bd/lifecycle.
