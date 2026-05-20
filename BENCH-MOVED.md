# BENCH-MOVED — `internal/bench/` left mindspec in spec 084

Spec 084 (`mindspec-otel-only`) deleted `internal/bench/` and the
`mindspec bench` cobra subtree from mindspec. mindspec is now a pure
spec/plan/lifecycle tool that points at any OTLP/HTTP endpoint via
`mindspec otel setup`; it does not own a benchmarking subsystem.

See [ADR-0027](.mindspec/docs/adr/ADR-0027-mindspec-otel-only.md)
(OTEL-only posture) and
[ADR-0028](.mindspec/docs/adr/ADR-0028-bench-rescue-procedure.md)
(this rescue procedure).

## Rescue procedure

The pre-deletion state of `internal/bench/` is preserved at the
annotated tag **`pre-spec-084-bench-delete`**. The tag is created
locally in the spec 084 PR branch; the push to `origin` happens as
part of the merge step (per HC #11 default option (b)), so the rescue
handle survives squash-merge of the spec 084 PR.

To lift `internal/bench/` into a new repository, or to inspect the
code from history:

```bash
git fetch origin --tags

# Recover the entire bench directory into the working tree:
git checkout pre-spec-084-bench-delete -- internal/bench/

# Inspect a single file without checking out:
git show pre-spec-084-bench-delete:internal/bench/runner.go

# Stream-format the deletion as a patch:
git diff pre-spec-084-bench-delete..HEAD -- internal/bench/
```

The `cmd/mindspec/bench.go` cobra command file is recoverable from
the same tag (`git checkout pre-spec-084-bench-delete -- cmd/mindspec/bench.go`).
The `cmd/agentmind-fake/` test fixture is likewise preserved at the
tag.

## Why this is not "extracted"

Extracting bench to its own repo is strictly slower than deleting it
with a one-command rescue note. The user has stated bench is "destined
for its own repo" — i.e., not mindspec's problem. The future
bench-repo author lifts the code from the annotated tag whenever they
want; mindspec stops carrying ~3,500 LOC of unowned subsystem in the
meantime.
