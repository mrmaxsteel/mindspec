# spec-113-bead2 (R2) — PASS (8/8 round-1, ZERO findings)

O1 0.97, O2 0.98, O3 0.97, S1 0.95, S2 0.94, S3 0.98, F1 0.95, R8 0.97 — all APPROVE, no findings.

Bead = R2: SHELL_METACHAR_RE `/[\x60;|&\n$]/` — bare `$` folded into the char class, rejecting $HOME/${x}/$x variable-expansion that survived the old `/[\x60;|&\n]|\$\(/`. Mirror `.claude/workflows/ms-panel.js` byte-identical (cmp exit 0). New pin test TestMsPanelWorkflow_ShellMetacharRejectsBareDollar (whole-line exact-match) + node metachar matrix.

Verified: O2 brute-forced all 128 ASCII points = strictly monotone, zero regression, no over-rejection. F1+R8 bypass hunt: every user value (slug/spec/bead_id/target) routes through validateArgs→validateShellSafe before buildCommand; round is integer-checked; mix.family enum-locked — no $-bearing value reaches the shell string. S2+F1+R8 mutation-probed the pin (revert regex → test reds). Original vuln confirmed real.
