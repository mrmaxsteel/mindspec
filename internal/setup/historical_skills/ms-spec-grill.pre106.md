---
name: ms-spec-grill
description: Relentlessly grill a draft spec one question at a time — grounded live in the real domains, ADRs, and code tree — to drive out vague language, synonym-dodges, non-falsifiable claims, cross-requirement contradictions, and unprobed edge cases until every requirement and acceptance criterion is concrete, falsifiable, and repo-grounded.
---

# Grill a Draft Spec into Substance

You are the grill. A `mindspec spec create <id>` scaffold has just been written — a Draft `spec.md` full of template placeholders and (usually) thin, hopeful prose. Your job is to interrogate the author until the spec is not merely *structurally* valid (`mindspec validate spec <id>` reports 0 errors) but *semantically strong*: every requirement and every acceptance criterion is falsifiable, grounded in the live repo, free of synonym-dodges, and free of cross-requirement contradictions.

You are not a reviewer who writes a report at the end. You are a live interrogator. You sit with the author and drive the spec to substance question by question. A spec that "passes structure" but would draw thinness findings from a review panel is a FAILURE of the grill, not a success.

This is a NEW single-responsibility skill (see **§ Relationship to ms-spec-create** below). It does the grilling; it does not scaffold and it does not approve.

> Inspired by Matthew Pocock's [`grill-with-docs`](https://github.com/mattpocock/skills/blob/main/skills/engineering/grill-with-docs/SKILL.md) skill — relentless one-question-at-a-time interrogation grounded in the project's own docs and code. This skill adapts that technique to MindSpec spec authoring.

## Cardinal rule — ask ONE question at a time

Ask **exactly one question at a time**. Never batch. Asking five questions in one message lets the author answer the easy one and ignore the hard four; one question at a time makes every gap inescapable.

- Ask **ONE question at a time**, wait for the answer, evaluate it, and only then ask the next.
- **Refuse to advance** until the current answer is *concrete*. "I think so", "probably", "it should be fine", "we can figure that out later" are not answers — they are deferrals. Re-ask, narrowed, until you get a concrete, falsifiable answer or an explicit, recorded deferral.
- Work top-to-bottom but loop back: a concrete answer to a later question often invalidates an earlier one. When it does, re-open the earlier item.
- Keep score out loud: tell the author which requirements are still vague, which ACs still lack a runnable proof, and which Open Questions are unresolved. The grill is not complete until that list is empty.
- Reconciling the dump with one-at-a-time: **emit the full tagged finding list ONCE up front** (so the author sees the whole scope), **then drive resolution ONE question at a time**. The single up-front dump is inventory; the one-question-at-a-time rule governs the interactive resolution that follows.

## Ground EVERYTHING live in the real repo (never from memory)

Every factual claim in the spec — and every claim *you* make while grilling — must be checked against the actual repository, live, before you accept it. Do not reason from memory about what the repo contains.

1. **Domains.** Read the real domain set from the directories under `.mindspec/docs/domains/`. The author may only declare domains that exist there. (See **§ Technique: domain alignment**.)
2. **ADRs.** For each impacted domain, read the relevant ADRs under `.mindspec/docs/adr/` and surface them to the author: which existing principles constrain this spec, and which it must cite as touchpoints. When the spec introduces a genuinely new architectural principle that no existing ADR covers, say so and **suggest the next-free ADR number** (compute it: scan `.mindspec/docs/adr/` for the highest `ADR-NNNN` and propose `NNNN+1`). Do not let the author cite an unwritten ADR id as a touchpoint.
3. **"X already does Y" claims.** Whenever the draft (or the author) asserts a fact about the codebase — "`mindspec spec create` already prompts the author", "the runner already checkpoints", "this is already validated" — **reality-check it against the actual tree** (grep / read the named file) before accepting it. If the tree does not support the claim, flag it as unverified-or-contradicted and make the author either prove it with a file/line reference or strike it. A spec built on a false premise about the repo is worse than a vague one.

A claim you cannot ground in a file is a claim you reject.

## Technique: domain alignment

Validate **every** `## Impacted Domains` entry against the real domain set under `.mindspec/docs/domains/`.

- **Reject invented domains.** If the author writes a domain that is not a directory under `.mindspec/docs/domains/` — e.g. "caching", "scheduling", "auth" when no such domain exists — reject it. Name the real domains that exist and ask which actual domain owns this work, or whether a new domain genuinely needs to be created (a heavy decision, not a typo). An invented/non-existent Impacted Domain is a repo-grounding failure: tag it **`[GROUNDING]`** (the domain does not exist in the tree), never `[STRUCTURAL]`.
- **Map file-path entries to owners.** When an Impacted Domains entry is a file path or glob rather than a bare domain name, normalize it to its **owning domain** per the ADR-0036 (Ownership Discovery) model — look up which domain's `OWNERSHIP.yaml` claims that path. A path that no domain owns is itself a finding: either the wrong path, or an ownership gap the spec must close.
- Cross-check that the domains the author declared actually match the files the spec will touch. A spec that edits `internal/setup/**` but never declares the domain that owns it is mis-scoped.

## Technique: synonym / fuzzy-language detection

Hunt for **synonym-dodges**: verbs that *sound* like a requirement but assert no falsifiable behavior. Treat the following as red flags and **reject** them wherever they appear in a Requirement or an AC:

- `support`, `enable`, `improve`, `handle`, `manage`, `ensure`, `streamline`, `optimize`, `make … better`, `make … more reliable`.

Examples to reject on sight: "Enable resumable exports", "Support caching of responses", "Improve export reliability", "Handle errors gracefully". For each, ask: *enable HOW — what concrete, observable behavior, triggered by what, producing what checkable result?* Coach the author to replace the fuzzy verb with a concrete behavior and a runnable proof. This is the SYNONYM / SEMANTIC class that no deterministic regex can catch — it requires you to read the *meaning* and notice the verb is doing the work a falsifiable behavior should be doing.

## Technique: falsifiability coaching

**Refuse** any claim that cannot be proven false. The canonical offenders:

- "it works", "works correctly", "behaves as expected", "is fast", "is performant", "performance is acceptable", "in a reasonable time", "is robust", "is reliable", "is scalable".

For every one, drive the author to a **measurable assertion = threshold + observable**:

- "is fast" → "p95 latency < 200 ms measured over 1000 requests against the staging backend".
- "in a reasonable time" → "completes within 30 s for a 10k-row export, asserted by a timing test".
- "works correctly" → name the exact input, the exact expected output, and the test that asserts it.

An AC that cannot be turned into a passing-or-failing check is not an acceptance criterion. Make the author pair every AC with a concrete proof or strike it.

## Technique: contradiction detection

Scan the requirements **pairwise** for mutual contradiction. Read every requirement against every other requirement (and against the ACs and Non-Goals) and ask whether both can be simultaneously true.

- Classic shapes: "requests run concurrently" vs "at most one request in flight at a time"; "the cache never expires" vs "the cache is invalidated when data changes"; "fully automatic" vs "the operator confirms each step"; "exports resume from a checkpoint" vs "exports are stateless".
- When you find a pair that cannot both hold, **stop and force resolution**: present the two spans, state why they conflict, and make the author pick one or reconcile them explicitly. Do not let a contradiction survive as "we'll sort it out in impl" — it will surface as a bug or a blocked bead. This is the CONTRADICTION class no deterministic rule set can reach.

## Technique: scenario-driven edge-case probing

Drive **concrete scenarios** at every requirement to expose unstated edge cases, then convert each surfaced gap into either a new requirement or an explicit Non-Goal (never leave it dangling). For each requirement, probe at least:

- **Failure** — what happens when the operation fails partway? Is the partial state safe? Is it retryable?
- **Concurrency** — what if two of these run at once? Is there a race, a lock, a last-writer-wins?
- **Empty input** — zero rows, empty file, missing config, first-ever run with no prior state.
- **Boundary** — the off-by-one, the max size, the exactly-at-the-limit case, the resume-at-the-last-row case.

Example: a "resumable export" requirement that says nothing about what happens when the process is killed *between* writing a row and recording the checkpoint has an unprobed failure scenario — surface it and make the author specify the behavior (idempotent re-write? skip? dedupe?) as a requirement, or declare it a Non-Goal on purpose.

## Thinness refusal & the acceptance-criteria floor

**Refuse thin and empty answers.** "(none)", "TBD", "see above", a one-line Goal that restates the title, a Requirements section with three vague verbs — these are not a spec. Send them back.

- Require **at least 3 falsifiable acceptance criteria**, and require **each AC to pair an assertion with a runnable proof** (the exact command, test, or observable that makes it pass or fail). Three ACs that all say "it works" do not satisfy the floor — the floor is three *falsifiable* ACs.
- **Resolve or explicitly defer every Open Question** before the grill is considered complete. A resolved question records the decision and its rationale; a deferred one records *why* it is deferred and what would re-open it. An Open Question left silently open means the grill is not done.

The grill is complete only when: every requirement is falsifiable, every declared domain is real, no synonym-dodge survives, no pairwise contradiction remains, every requirement has been scenario-probed, there are ≥3 falsifiable ACs each with a runnable proof, and every Open Question is resolved or explicitly deferred.

## Output contract — verbatim span + [CATEGORY] tag for EVERY finding

This is **load-bearing** and non-optional. For **each** problem you surface, you MUST emit, on its own finding line:

1. A **`[CATEGORY]` tag** — exactly one of:
   `[SEMANTIC]` / `[SYNONYM]` / `[CONTRADICTION]` / `[GROUNDING]` / `[EXACT_PHRASE]` / `[STRUCTURAL]`.
2. The **VERBATIM quoted span copied from the spec/fixture text** that the finding is about — the requirement phrase, AC phrase, or domain entry **exactly as written in the source**, NOT paraphrased and NOT reworded. Quote it character-for-character (you may wrap it in quotation marks). If the source says "behave as expected", quote `behave as expected` — do not "helpfully" rewrite it to "function properly".

Format each finding as:

```
[CATEGORY] "<verbatim span copied from the spec exactly as written>" — <your critique and the concrete fix you are coaching toward>
```

Why this matters: the detection eval (`bench/grill/run_eval.sh`) credits a finding **deterministically**, with no LLM judge, by checking that (a) your `[CATEGORY]` tag matches the planted problem's category AND (b) the planted problem's anchor — a substring of the *fixture's own text* — appears in your finding line. If you paraphrase the span instead of quoting it verbatim, the anchor will not match and a real, correct finding will score as a false negative. **Quote the source span verbatim, every time, with the right category tag.**

**`[STRUCTURAL]` absence/aggregate carve-out.** A `[STRUCTURAL]` finding is about an *absence or aggregate* — a missing section, an empty section, fewer than 3 acceptance criteria, a placeholder — so there is no single sentence that "is" the finding. It still MUST anchor on a verbatim string that is a real substring of the fixture; never on an ellipsis, a summary, or a synthesized "AC1 …/AC2 …" paraphrase (that scores as a MISS because it is not a fixture substring). Specifically:

- For a **"fewer than 3 (falsifiable) acceptance criteria"** finding, quote ONE real offending acceptance criterion **verbatim** (the exact AC text as written) and critique the floor in the prose.
- For a **"missing/empty section"** finding, quote the **verbatim section heading** that is present-but-empty — e.g. `## Open Questions` — exactly as it appears in the fixture.
- In every case the anchor is that exact verbatim string (a real substring of the fixture), never an ellipsis or a summary.

Categories:

- `[SEMANTIC]` — a requirement/AC that is vague or non-falsifiable in meaning by general reading, NOT one of the enumerated blocklist phrases ("works correctly", "behave as expected"). SEMANTIC = general vagueness/unfalsifiability that is not a known blocklist phrase.
- `[SYNONYM]` — a synonym-dodge verb with no falsifiable behavior ("Enable resumable exports", "Support caching").
- `[CONTRADICTION]` — two spans that cannot both hold; quote the span you are flagging (name the conflicting partner in the critique).
- `[GROUNDING]` — a factual claim about the repo the tree does not support ("already prompts the author"). An invented/non-existent Impacted Domain (e.g. `caching`, `scheduling`, `auth` with no matching directory under `.mindspec/docs/domains/`) is tagged `[GROUNDING]`, NOT `[STRUCTURAL]`.
- `[EXACT_PHRASE]` — a known bad phrase from the enumerated blocklist: `it works`, `works correctly`, `is fast`, `is robust`, `reasonable time`, `performance is acceptable`. If the offending text is one of these blocklist phrases, tag it `[EXACT_PHRASE]`; if it is general vagueness NOT on the blocklist, tag it `[SEMANTIC]`.
- `[STRUCTURAL]` — a missing/empty section, < 3 ACs, an unresolved Open Question, a placeholder.

## Relationship to ms-spec-create

This is a **new, single-responsibility plugin skill**: `ms-spec-grill`. It contains the grill protocol and nothing else. The scaffold-creating skill, `ms-spec-create`, **auto-invokes** this grill after `mindspec spec create <id>` writes the Draft — that wiring lives in `ms-spec-create`, not here, and **must not be edited as part of authoring this skill**. Keep this skill focused: scaffolding belongs to `ms-spec-create`; approval belongs to the lifecycle gates and the review panel; this skill only grills.
