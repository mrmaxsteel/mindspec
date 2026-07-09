# wu7t-finalize-fix — round 2 consolidated outcome

**Decision: PASS (5 APPROVE / 1 REQUEST_CHANGES / 0 REJECT; threshold 5/6; no hard_block; reviewed_head_sha 6cf718c9 fresh).** Merge proceeds.

All round-1 concrete_changes_required were verified ADDRESSED by their original raisers (R4: retry idempotency incl. an independent scratch-fixture probe of the observed-SHA lease, which also correctly rejects third-party tip movement; R5: ordering/self-heal/fallback all pinned by the new tests; R6: NOTE composition gated + tested, merge-tree clean against current origin/main). Round-1 approvers confirmed no regressions (R3: all seven ordering contracts hold; R2: full uncached suites green, exactly two commits ahead).

## R5's new round-2 asks (non-blocking, deferred → appended to mindspec-3xqm)

1. `RemoteHeadSHA`: `git ls-remote --heads <pattern>` does SUFFIX matching — a decoy ref (e.g. `refs/heads/aaa/chore/finalize-105`) also matches and can sort first; `strings.Fields(out)[0]` takes it silently. Fail-safe today (the lease then rejects; specID format makes it unreachable), but cheap to harden with an exact-refname match on the second ls-remote column.
2. `PushBranchForceWithLease` doc overclaims "never silently clobbered" — a HUMAN commit already on the machine-owned `chore/finalize-<specID>` branch (the branch the NOTE tells operators to open a PR from) is force-overwritten by design on a later retry. Correct the doc comment; consider whether the reconcile should diff before overwriting.
