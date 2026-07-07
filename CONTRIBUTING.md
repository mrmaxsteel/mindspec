# Contributing to MindSpec

Thanks for your interest in MindSpec! Contributions are welcome — code, docs, bug reports, and real-world friction reports alike.

One thing to know up front: **MindSpec is built with MindSpec.** Every feature in this repo has shipped through the spec → plan → implement → review lifecycle, with panel-review verdicts committed alongside the code (browse `.mindspec/specs/` to see the full history). Contributions ride the same rails, though how much of the lifecycle you touch depends on the size of the change.

## Bug reports and ideas

Open a [GitHub issue](https://github.com/mrmaxsteel/mindspec/issues). For bugs, include the output of `mindspec version` and `mindspec doctor`, what you ran, and what happened. For security issues, **don't open a public issue** — see [SECURITY.md](SECURITY.md).

If you're a MindSpec user, `mindspec report list` is a goldmine: friction reports (where you hit overrides or escape hatches) are exactly the feedback that drives the roadmap. The journal is local and redacted by design — nothing is sent anywhere — so anything you choose to share from it, you share deliberately.

## Small fixes

Typos, small bugfixes, doc corrections — the standard GitHub flow:

1. Fork and branch: `git checkout -b fix/<short-description>` (never commit to `main`; the repo's hooks will refuse anyway).
2. Make the change. `make test` must pass; run `golangci-lint run` if you touched Go code.
3. If you changed behavior under `cmd/` or `internal/`, update the matching docs — doc-sync is a gate in this repo, and PRs that let code and docs drift won't pass review.
4. Open a PR with a clear description of the defect and the fix. One logical change per PR.

Expect your PR to be reviewed by a panel as well as a human — MindSpec dogfoods its own review machinery on incoming changes. The verdicts you get back are structured findings; "concrete changes required" means exactly that.

## Features and larger changes

**Open an issue to discuss before writing code.** MindSpec is an opinionated framework, and the opinions are load-bearing: features that weaken a gate, add an unaudited escape hatch, or move a rule from the binary into a prompt will be declined regardless of code quality. The design principles in the [README](README.md#design-principles) and the ADRs under [`.mindspec/adr/`](.mindspec/adr/) are the constitution — a feature that needs to deviate from an ADR needs a superseding ADR, which is a conversation worth having *before* the implementation exists.

Once a feature is agreed, it goes through the lifecycle: a spec (with falsifiable acceptance criteria — expect it to be grilled), a plan, and bead-sized implementation. If you have MindSpec installed you can run the lifecycle in your fork yourself; otherwise a maintainer will carry your proposal through it with you.

## Development setup

```bash
git clone https://github.com/mrmaxsteel/mindspec
cd mindspec
make build      # builds ./bin/mindspec  (requires Go 1.23+)
make test       # runs all tests
```

You'll also want the [Beads](https://github.com/gastownhall/beads) CLI (`bd`) on your PATH — the test suite and the lifecycle both use it.

A few conventions the codebase holds itself to:

- **Gates fail closed and exit non-zero**, ending with a machine-greppable `recovery:` line. New validations follow that pattern.
- **State is derived, never cached** — lifecycle phase comes from beads + git. Don't introduce session-resident state.
- **Escape hatches are journaled** — any new override flag must record its use to the friction journal.
- **Tests must pass, actually** — "tests pass" claims in PRs are re-run by the empirical-prober review lens, so save everyone a round trip.

## License

MindSpec is [MIT licensed](LICENSE). By contributing, you agree that your contributions are licensed under the same terms.
