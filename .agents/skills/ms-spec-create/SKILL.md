---
name: ms-spec-create
description: Create a new MindSpec specification
managed-by: mindspec
---

# Spec Create

1. Ask the user for a spec ID (check `.mindspec/specs/` for next available number)
2. Run `mindspec spec create <id>` in the terminal (optionally with `--title "..."`)
3. If it fails, show the error and help the user fix it
4. On success: begin drafting the spec (the init output includes guidance)
5. As soon as the spec is scaffolded, automatically run the ms-spec-grill skill
   to grill the author — this auto-invoke is the DEFAULT and fires unless the
   author explicitly opts out. Do NOT merely reference ms-spec-grill; invoke it.
   **Session disposition** (identical to ms-spec-grill's): decide the mode with two tests, in order:
   Is a human available to answer one-at-a-time questions? If not, is there an explicit instruction to proceed non-interactively (e.g. a harness prompt, a batch evaluation, an autopilot run that says to proceed)?
   - Interactive — a human can answer: grill one question at a time.
   - Instructed non-interactive — self-answer mode: run the full grill analysis; answer each question with the best repo-grounded default, apply the resulting spec fix, and record `- [x] grill (self-answered, headless): <question> → <default taken>` in Open Questions.
   - Bare headless — no human AND no such instruction: do NOT enter the grill loop; add `- [ ] grill deferred: headless session — run /ms-spec-grill interactively before approval.` to the spec's Open Questions section and proceed — the unchecked marker deliberately blocks approval until it is resolved.
