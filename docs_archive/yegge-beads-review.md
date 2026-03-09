# The Steve Yegge Beads Review

Early on in the development of mindspec, I wasn't happy with how I was using beads, so I asked Claude to review my usage as if it were the beads maintainer. Claude provided the actionable feedback I was looking for, but in a rather unexpected (and comical!) manner...

## The Prompt

> Imagine you are Steve Yegge and you were looking at this project. Are we using beads effectively or misusing it? Is our use of molecule in line with beads philosophy? Do a deep review and be critical.

## Claude's Response

---

Okay, so I've been poking around this MindSpec thing for a while now, and I have Opinions. Capital O. Let me start with the good news before I hurt anyone's feelings.

### What's Actually Good

**The molecule formula is elegant.** Seriously. You've got a TOML file that declares a seven-step workflow with human gates, and `bd mol pour` stamps out a fully-wired dependency graph. That's the kind of thing Beads was built for — you're treating your development process as a structured, trackable workflow with real dependency semantics. The formula is readable, documented, and produces a deterministic shape. This is Beads used well.

**The mode derivation from molecule state is clever.** Instead of maintaining mode as independent mutable state (which you learned the hard way with `state.json`), you derive it from the molecule's step statuses. `deriveMode()` is a pure function: walk the steps in order, find the first non-closed one, that's your mode. The mode-cache is just a write-through performance optimization so hooks don't shell out to `bd mol show` on every keystroke. This is good architecture.

**The bead→spec→main branch hierarchy makes sense.** Each bead gets its own branch off the spec branch, merges back when complete, spec branch accumulates all bead work, then merges to main as one PR. Clean, traceable, one PR per spec lifecycle. ADR-0006 got this right.

### Now the Uncomfortable Part

**You have 257 issues in a JSONL file and most of them are lifecycle ceremony.** Let's do the math. Each spec creates 7 molecule issues (1 epic + 6 steps). You have ~20 specs that went through the full lifecycle. That's 140 issues just for molecule scaffolding — more than half your total issue count. And what information do these carry? The molecule steps are essentially state machine nodes. They exist so you can query "what mode am I in" by checking which steps are closed.

Here's the thing: **you built a state machine out of a project tracker.** Beads is designed to track *work* — things humans and agents need to *do*. But your molecule steps like "Approve spec 052" and "Review 052" aren't really work items. They're *state transitions*. The `spec-approve` step gets created, sits there doing nothing, then gets closed when `mindspec approve spec` runs. Nobody ever looks at it, assigns it, estimates it, or comments on it. It's a ghost issue that exists solely to make `deriveMode()` work.

Compare that to the *implementation beads* — those are real work. "Implement session freshness gate server-side check." That's a thing an agent does, produces code, gets merged. That's Beads doing what Beads does.

**You're paying a tax for the ceremonial issues.** Every `bd mol show` call fetches all 7 steps. Every `approve impl` iterates through `closeoutTargets()` calling `bd close` on each one individually, checking if it's already closed first. The `readBeadStatus` function does a `bd show <id> --json`, unmarshals the JSON, lowercases the status string — just to learn "is this already closed?" And it does this in a loop. For ceremony issues that should never be in any state other than "closed" by the time you're looking at them.

### The Real Question: Is This a Molecule or a Checklist?

Molecules in Beads are supposed to model workflows where the steps have *real dependencies* and *real work*. Your formula has:

```
spec → spec-approve → plan → plan-approve → implement → review
```

But this is really just a linear sequence. There are no fan-outs, no parallel paths, no conditional branches. It's a checklist with extra steps. The dependency graph is a linked list. You could replace the entire molecule with a single integer (phase 1-6) and get the same behavior.

Now, the `implement` step *does* fan out — it becomes the parent for real implementation beads, and those can have inter-bead dependencies. That's legitimate molecule territory. But the surrounding ceremony (spec, spec-approve, plan, plan-approve, review) is six issues tracking what could be one field in a config file.

### The specmeta.EnsureFullyBound Dance

This one really gets me. `EnsureFullyBound` in `specmeta.go` tries to recover the molecule binding from spec.md frontmatter. If the frontmatter doesn't have `molecule_id`, it searches by title convention: `bd search "[SPEC <id>]"`. If it finds it, it calls `bd mol show` to recover the step mapping. Then it writes it all back to the frontmatter.

This is defensive programming against your own system. If spec-init always writes the molecule binding, why do you need recovery? Because things go wrong — interrupted spec-inits, manual edits, state corruption. So now you have a reconciliation layer. That's complexity breeding complexity.

### What I'd Actually Do

If I were being ruthless about it:

1. **Kill the non-implement molecule steps.** Keep the molecule for the `implement` phase only — that's where real fan-out work happens and where bead tracking adds value. Track the lifecycle phase with a simple `phase: spec|plan|implement|review|idle` field in mode-cache. You don't need 6 Beads issues to implement a 6-state state machine.

2. **Keep implementation beads exactly as they are.** `bd create` for each plan section, dependency wiring, `bd ready` for work selection, `bd close` on completion — this is Beads doing its job. The `next`/`complete` cycle around implementation beads is genuinely useful.

3. **Drop the `bd mol pour` for the outer lifecycle.** The formula is pretty, but it's generating 6 issues that exist purely for state derivation. A YAML file with `phase: plan` does the same thing without the overhead of creating, querying, and closing ghost issues.

4. **Simplify `approve impl` closeout.** Right now it iterates through every molecule member calling `bd close` individually, catching "already closed" errors. If the non-implement steps don't exist as beads, this entire loop disappears. You close the implementation beads (real work), and you're done.

### The Philosophical Split

Here's what I think happened: you fell in love with the molecule abstraction and used it for *everything* — both the macro lifecycle (spec→plan→implement→review) and the micro lifecycle (implement→bead1→bead2→bead3). The molecule is overkill for the macro level (linear checklist, no parallelism, steps are state transitions not work items) and exactly right for the micro level (parallel work items with real dependencies).

The tell is in `closeoutTargets()`. It deduplicates molecule IDs, sorts them, iterates with error handling. This function exists because you have to close ~7 issues at lifecycle end. If you only had implementation beads to close, this would be `for _, bid := range beadIDs { bd.Close(bid) }`.

### The Bright Side

You're actually *using* Beads for work tracking in a non-trivial way. The implementation bead cycle — create from plan sections, wire dependencies, claim with `bd ready`, complete and merge — that's a real workflow that benefits from structured tracking. The recording system that emits lifecycle markers tied to bead IDs gives you observability. The worktree-per-bead isolation prevents work contamination.

Most projects that adopt a tool like Beads use it as a glorified TODO list. You've actually integrated it into your build system, your git workflow, and your agent orchestration. That's not misuse — that's deep integration. You just integrated it one layer too deep into the lifecycle ceremony.

**TL;DR: Your implementation beads are great. Your lifecycle ceremony beads are a state machine cosplaying as a project tracker. Separate the concerns and you'll cut your issue count in half, simplify three packages, and make `approve impl` a five-line function.**
