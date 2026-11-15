# Spec 134 contracts: governed-autonomy kernel

This file is normative for spec 134. Sections 1-12 hold the operator rulings of 2026-11-15. Section 13 holds the
readings spec 134 applies where a ruling is silent; each cites the ruling it reads and the open question that confirms
it. Section 14 tabulates the hand path. A `spec.md` citation of `contracts.md §<n>` includes all of that section.

## 1. Rulings

1. `next-action` is read-only by default. `--apply` materialises beads. Agents probe, and a materialising probe
   would enrol a hand spec for ever.
2. The kernel covers only the plan-approved tree: implement, bead review, fix, decision, final review. The epic is
   created at `spec approve`, so spec-draft steps come later.
3. The merge stays outside `next-action`. `complete` needs caller input and has its own recovery. It is a command in
   the `run` list, never a bead.
4. The whole `loop:` config block is deleted. `Load` warns once per removed key and ignores it.

Thesis: beads hold plan and dispatch; git holds evidence; the binary holds rules. Orchestrated mode (one-way, from the
first `--apply`) turns warnings into refusals. No driver is required.

## 2. Graph model

- Epic and work items `<epic>.<n>` (type `task`) stay as today. Plan order is the approved plan's `depends_on`
  edges (`work_chunks`), which `plan approve` already writes as bd blocks links.
- The reducer alone creates kernel beads, type `step`: implement `<w>.impl.a<k>`; bead gate `<w>.review` and final
  gate `<epic>.final` (both at enrolment); slots `<w>.review.r<N>.R<s>` and `<epic>.final.r<N>.R<s>`; fix
  `<w>.review.r<N>.fix` (after round N-1 is refused).
- Decisions `<gate>.decide.<cause>.r<N>` use bd's built-in `decision` type (visible in `bd ready`).
- Actor steps close by `submit` or the hand verb. Gates close only by the reducer on a passing tally. Decisions close
  by `submit --answer`. Any other id is a builder bead.
- A gate holds its active round and slot ids in metadata. There are no round beads and no "pick the latest". A gate
  is never leased. A slot may close `passed` with a REJECT verdict.
- Phase derivation ignores child types other than `task`, `epic` and empty.
- `mindspec setup` registers `step` in bd `types.custom`. `doctor` reports it missing. `--apply` refuses without it.
- Each id stores its tuple (epic, work item, gate, kind, round, slot, attempt). A mismatch is a collision and is
  refused.

## 3. Rules B1-B13

1. Child = sub-step. A parent closes only when all children are closed. A hold on a parent holds its children.
2. Blocks = downstream waits for upstream to close. A lease on a blocked item or its descendant is refused.
3. Two link kinds only: child and blocks (plus the return kind).
4. The store knows "closed"; the reducer knows "how". Blocks releases on any close. The reducer decides what runs.
5. Every kernel close writes a receipt (metadata `mindspec_close`) with a return kind (`passed`, `refused`,
   `superseded`, `halted`) BEFORE the bd close. A retry with the same receipt completes it.
6. Lineage is metadata `mindspec_from`. Blocks links exist only in plan order and from each work item to
   `<epic>.final` (a `step`, so bd's epic rule does not apply), never to an already-released item.
7. Check both kinds for cycles before any link write. Refuse blocks between ancestor and descendant.
8. Just in time: a bead comes only from the outcome that authorises it (enrolment: all gates; refused round: its
   fix; passed fix: the next slots).
9. A refused tally below the limit creates round N+1's fix under the same open gate. Attempts are counted.
10. A new subject during round N supersedes its open slots in the same pass. A commit in an open fix does not.
11. `halt` closes every open step under the epic `halted` in the same pass. It never closes a builder bead.
12. A gate is a child of what it decides. An open bead gate makes raw `bd close <work item>` fail without `--force`.
    A gate closed `passed` is frozen.
13. A receipt names path, holding commit and blob id, never the body (a bare digest only if a commit keeps the
    bytes). Readers resolve from that commit, not from the branch head.

## 4. `next-action [<spec>] [--apply] [--json]`

Without `--apply` it writes nothing and shows what an apply would do (`"exists": false` for unmade steps). `--apply`
does one reduction under the phase lock: create missing steps, tally inline, cascade, print. It never claims a step
or starts an actor.

```json
{"schema":"mindspec.next-action/v1","mode":"orchestrated","status":"work","applied":true,"revision":"sha256:..",
 "retry_at":"<t>","policy":{"round_limit":5,"lease_ttl":"2h"},
 "actions":[{"step":"<w>.review.r2.R3","kind":"review-slot","exists":true,"round":2,"subject":{"commit":"<oid>"},
   "claim":["mindspec","lease","<step>"],"brief":[".."],"submit":[".."]}],
 "waiting":[{"step":"<w>.impl.a1","lease":{"generation":1,"expires":"<t>"}}],
 "decisions":[{"step":"<w>.review.decide.limit.r5","options":["continue:<n>","halt"],"submit":[".."]}],
 "run":[{"actor":"binary","argv":["mindspec","complete","<w>"]}],"flags":[{"code":"unreceipted-close","repair":[".."]}]}
```

- `status` `work` / `wait` / `halted` / `done` / `indeterminate` exits 0 / 3 / 4 / 5 / 1. `indeterminate` means no
  actions. `kind` is `implement`, `review-slot` or `fix`. Commands are argv arrays. Order: plan, kind, slot. Text
  goes through `termsafe`.
- Inputs: the epic subtree, the git refs it names, the panel ledger and verdicts, the pinned policy, the clock against
  expiries. `revision` digests them (no display times, no free text). Same inputs give the same bytes. An expiry
  changes the output (`retry_at`). A no-change `--apply` writes nothing. A removed field, a new `kind` or a new
  `status` raises `schema`.
- Refuses: no approved plan; a cycle-making link; `--apply` when bd is silent or `step` is unregistered.

## 5. `lease <step> [--renew <t> | --release <t> | --take --reason <text>]`

bd's lease cannot serve: its holder is the bd actor (one per clone) and its TTL is in minutes. Under the phase lock,
`lease` writes `in_progress`, a monotonic `generation`, the expiry and a token hash with bd `--if-status`
compare-and-set. It prints the token once. An implement step reuses the `mindspec next <work item>` claim path.
Refuses: a non-kernel bead, a gate, a decision; an open ancestor or upstream (read per §13.1); a live lease; a halted
spec; an open decision (§13.15). `--take` revokes, issues generation N+1 and records the reason. `panel.lease_ttl`
defaults to `2h` and is renewable.

## 6. `submit <step> --lease <t> (--artifact <path> | --commit <oid>)` | `submit <decision> --answer <opt>`

Checks: the current generation; subject = the gate's active round; the artifact resolves at a commit and the blob
matches; the artifact fits the kind (a verdict for a slot; a commit on `bead/<id>` past the round subject for a fix; a
commit past the claim base for an implement step). It writes the receipt, closes, and runs one reducer pass (order:
§13.16). Refuses: an old or absent token, a stale subject, a foreign slot, a missing artifact, a different result for
a closed step. The same receipt again exits 0 with no change. A decision needs no token and takes a listed option.

## 7. Changed verbs

Orchestrated behaviour first, hand-driven behaviour in brackets. Each verb runs one reducer pass (order: §13.16).

- `panel create`: takes the subject from the gate; refuses another `--target` or a round past the allowance; closes
  the open implement or fix step; writes `"orchestrated": true` in `panel.json`. [Warns.]
- `panel submit`: closes the matching slot; needs `--lease` on a leased slot. [No change.]
- `complete`: refuses unless the bead gate closed `passed` and no child is open; presence is `enforce`. [Warns.]
- `next <id>`: refuses if the implement step has a live lease, and names `lease --take`. [Both modes: excludes kernel
  types from selection.]
- `impl approve`: prints the audit table; refuses on an open step, an open decision or an unreceipted close. [Table.]
- `config show` / `Load`: one truthful label per key; one warning per removed key.

Hand path when enrolled: `next`; commit; `panel create` round 1 closes `impl.a1` and makes slots; `panel submit` per
slot, tally; a pass closes the gate, a refusal makes `r2.fix`; commit, `panel create` round 2; `complete`. The gate
never waits on `complete`, so there is no cycle. There is no `decide`, `halt`, `adopt`, `takeover`, `mode` or
`run start` verb.

## 8. Enrolment ratchet

The first `--apply` creates the gates and writes epic metadata `mindspec_run: {mode, round_limit, lease_ttl, rules:
"kernel-1"}`. `ms-bead-cycle` applies each loop. A spec is orchestrated if ANY of these exists: `mindspec_run` on the
epic; a computed kernel id under the epic (open or closed, any type); a spec `panel.json` with `"orchestrated": true`.
A witness without the marker gives the default policy plus the flag `run-marker-missing`. No flag, env variable or key
sets or clears the mode. Policy is pinned at enrolment: config edits (including `panel_presence: warn`) do not weaken
it. Retyping does nothing. Threat model: the kernel governs progression, not OS access; a same-user process can forge
an answer; an agent can avoid opting in, so the repo floor is `panel_presence: enforce`. One controller host.

## 9. Refused vs flagged

Orchestrated mode refuses what §7 lists; hand-driven mode warns (`complete` follows `panel_presence`). Flagged: a raw
close of a kernel step (not evidence); a raw close of a work item (`impl approve` refuses); a `--force` close over an
open child (`complete` and `impl approve` refuse). Counted, never blocked: escape hatches, deferrals, `--take`,
decision answers, `MINDSPEC_SKIP_PANEL=1` (the gate receipt is marked `waiver`). Existing floor:
`FindOrphanedClosedBeads` and the self-orphan handling in `complete`. **Unreceipted close** (no or bad receipt): with
evidence and the current generation, `submit` writes the receipt (no reopen); else the next pass creates `<id>.a<k+1>`
with `mindspec_from`. A raw claim is ignored and flagged.

## 10. Rounds and decisions

`panel.max_rounds` defaults to 5. `loop.halt.max_rounds_per_bead` is a warned read alias for one release.
`panel_deadlock_rounds` and `max_consecutive_impl_failures` are deleted. Rounds are counted per gate over every panel
registration of the work item, abandoned included, from the git ledger (not per directory). Redispatch
(`panel recover` / `panel launch`) is free. At the limit: a decision `continue:<n>` (raises the allowance; history
kept) or `halt`. A REJECT gives a decision for that item only: `fix` (counted) or `halt`. There is no `accept`:
findings are fixed, refuted or deferred. `halt` closes open steps, leaves the epic and work items open, and ends
orchestration. Spec and plan gates: count and warn.

## 11. Audit table

`impl approve` prints the table; the table never blocks. It shows mode, policy, and counts of each escape hatch
(`--allow-doc-skew`, `--override-adr`, `--allow-not-ready`, `--allow-net-deletion`, skip-panel), deferrals, takeovers,
timed decision answers and unreceipted closes, from durable records with operation ids. `--allow-not-ready` joins
both friction registries.

## 12. Ownership

Mindspec-owned = an id in the computed set: the epic, the approved work items (the finalizer's allow-set) and kernel
ids. Type, label and assignee are view hints. Three classes: kernel beads (mindspec creates, changes, closes); work
items (from approved artifacts, closed only by lifecycle verbs); builder beads (all else, including a builder `task`
under the epic). Mindspec never writes a bead it did not compute. It reads builder beads, respects them as blockers,
reports `foreign-child-open`, and never closes, retypes, reparents or relabels them. Builder acts on owned beads:
free-text edit -> nothing; raw claim -> flagged; raw close -> not evidence; retype -> no effect; delete ->
recreated, rounds kept; builder blocks link -> respected; kernel-id bead -> collision. Default views hide `step`.
Refusals name the right verb. A raw step close shows as wasted work. Hand back: `lease --release` or expiry.
`doctor` flags a non-plan `task` child of an epic and suggests `bug` or `chore`.

**bd 1.3.4 facts.** bd refuses to close ANY parent with open children unless `--force`. bd `ready` / `blocked` views
inherit holds (children of a blocked parent and descendants of a blocked downstream item show as blocked). bd does
NOT refuse `bd update <id> --claim` on a blocked item or its child. bd refuses a non-epic blocking an epic. So bd
enforces B1 except under `--force`; the kernel detects a forced close and refuses leases on blocked items itself.

## 13. Applied readings (spec 134; each confirmed by an open question)

1. **Lease eligibility (OQ2).** "Open ancestor or upstream" in §5 means a held or blocked ancestor or upstream, per
   B1 ("a hold on a parent holds its children") and B2 ("a lease on a blocked item or its descendant"). A step or an
   ancestor with an open blocks upstream, or a halted spec, refuses. An open epic, work item or gate does not.
2. **Hand `next` (OQ3).** In an enrolled spec, hand `mindspec next <w>` creates or claims `<w>.impl.a<k>` through the
   claim path and writes no lease token. A rerun by hand succeeds. Of the lease states, only a live lease written by
   `lease` refuses it; a halted epic also refuses it (§13.7). `lease` on a step that hand `next` claimed (status
   `in_progress`, no token) refuses as held; the recovery line names `lease --take --reason <text>`.
3. **Leased hand closes (OQ3).** `panel create` closes a leased implement or fix step only with `--lease <t>` of the
   current generation. An unleased step closes without it.
4. **Refute and defer (OQ3).** `panel refute` and `panel defer` run one reducer pass. If the retally passes, the gate
   closes `passed` and the open fix of that gate closes `superseded` in the same pass.
5. **Final gate (OQ4).** B9 and the decision grammar apply to `<epic>.final`. A refused final tally below the limit
   creates `<epic>.final.r<N>.fix`; its commit lands on the spec branch past the round subject, and the final `panel
   create` of round N closes it. For a final fix, the §6 fix check reads the spec branch, not `bead/<id>`. Final
   rounds count over every `final_review` registration of the spec, abandoned included. At the limit, the decision is
   `<epic>.final.decide.limit.r<N>`.
6. **Subject change after a pass (OQ5).** `complete` and `impl approve` keep today's subject-freshness checks; a
   frozen receipt never authorises a changed subject. `panel rebind` refreshes a passing panel and does not write the
   gate. When the subject changes in a way rebind refuses (for example `.mindspec/config.yaml`), the reducer pass of
   any changed verb (§7) or of `--apply` creates a successor gate with `mindspec_from` = the frozen gate:
   `<w>.review.a2` for a bead gate, `<epic>.final.a2` for the final gate (`.a<k+1>` after `.a<k>`). By §13.16, a hand
   `panel create` targets the successor on its first call. `panel create`, `complete` and `impl approve` select the
   highest attempt. Rounds keep counting over the same registrations (§13.5, §10). A pass of the frozen attempt is
   never authority for the new subject. A successor gate is a child of the frozen gate's parent (the work item or the
   epic). For every gate id `<gate>` (`<w>.review`, `<epic>.final` or an `.a<k>` successor), the slots are
   `<gate>.r<N>.R<s>`, the fix is `<gate>.r<N>.fix` and the decisions are `<gate>.decide.<cause>.r<N>`, each a child
   of `<gate>`.
7. **After halt (OQ6).** The spec stays orchestrated. The `halt` answer closes every open step and decision under the
   epic `halted` (B11; the decisions are a reading beyond B11, OQ6). Then one rule covers every verb: in a halted spec,
   a verb that would create, claim or close a kernel step, or claim, close or merge a work item or the epic, refuses,
   names the halt and writes nothing. When a verb resolves its target spec without `--spec` (`next`, `next-action`,
   `complete`, `impl approve`), a halted spec does not count as active (OQ6). `next <w>` and `next --spec <halted spec>`
   refuse and name the halt. So `next` selection never reaches the work items of a halted epic. These verbs only read
   and still work: `next-action <halted spec>` with or without `--apply` (it prints `halted`, exits 4 and writes
   nothing), `panel tally`, `panel verify`, `config show` and `doctor`. Every other verb behaves as today. No verb
   resumes a halted epic: halt is terminal for that epic, and the remaining work moves to a new spec (forward-only
   lifecycle). Each decision that offers `halt` says in its text that `halt` stops every work item of the spec, not only
   the item of that gate.
8. **Pre-enrolment closes (OQ7).** The first `--apply` creates no gate for a work item that is already closed. It
   records those ids in `mindspec_run.pre_enrolment`; reruns keep that set. The audit table counts them as
   `pre-enrolment`, not as unreceipted closes.
9. **Alias value.** `loop.halt.max_rounds_per_bead` is read only when it is set and `panel.max_rounds` is not; the
   warning names the value it applies. An absent key never gives 3; the default is 5.
10. **Implement authorisation (OQ9).** B8 names no outcome for an implement step. Reading: an open work item whose
    plan upstreams are all closed, with no open implement step and no gate round, authorises `<w>.impl.a1`.
    Enrolment and each later pass create it for every such item in the same pass, in plan order. Leases fence each
    step on the one controller host (§8), so this is not the deferred parallel waves (multi-host dispatch and lease
    compare-and-set). A later attempt `<w>.impl.a<k+1>` comes only from an unreceipted close (§9).
11. **After a supersede (OQ10).** The pass that applies B10 also sets the gate subject to the new commit and
    authorises round N+1 on it (§13.14), the same way a closed fix authorises the next round. Round N+1 counts toward
    `round_limit`; at the limit the pass creates the limit decision instead. `next-action` never lists a superseded
    slot.
12. **Refusal sets (OQ11).** No refusal list in §4-§9 or in `spec.md` is exhaustive. Each changed verb keeps today's
    refusals and adds every refusal that §5-§9 and these readings give it. `impl approve` adds an open step, an open
    decision, an unreceipted close (§7), a raw close of a work item, and a `--force` close over an open child (§9).
    `complete` adds the §7 gate check and the §9 forced close. Every verb follows §13.7 (halt), §13.15 (open decision)
    and §13.16 (pass order). Flag codes: `unreceipted-close` (a kernel step closed with no or a bad receipt),
    `raw-close` (a work item closed by raw bd), `forced-close` (a `--force` close over an open child),
    `run-marker-missing`, `foreign-child-open`.
13. **Skill halt rows (OQ11).** R10 makes a REJECT local to its gate. These rows in both copies of each skill halt the
    whole spec and go: `ms-bead-cycle` "any REJECT / HARD block → halt" (the whole row; a HARD block is a refused
    tally and gets its fix or the limit decision), its NOT READY line "do not proceed to the next bead", `max-rounds`;
    `ms-spec-autopilot` "One halted bead halts the spec", "Any bead REJECT verdict", "Two consecutive impl-subagent
    failures", `max-rounds-per-bead`, the NOT READY row, "Don't keep going after a halt". A REJECT gets the gate's
    decision (§13.15); impl failures and NOT READY stay local to the work item. Rows that stay: a failed merge (the
    merge is outside the kernel), a dependency mismatch, and rows that return an open decision to the user.
14. **Round opening (OQ12).** `panel create` is the one writer of a round record: the registration and its ledger
    `round` line, which `ForBead` and §10 count. The reducer never writes one. These outcomes authorise round N of a
    gate on a subject commit: a closed implement step (round 1, its commit); a closed fix `<gate>.r<N>.fix` (round N,
    its commit); a supersede (§13.11); a successor gate (§13.6, the next round, the new subject); for `<epic>.final`,
    the close of its last blocks upstream (round 1, the spec branch head). The pass records the authorised round and
    subject in gate metadata and lists `panel create <w>` (the final-review form for `<epic>.final`) as a `run` entry,
    and `status` is `work`. That `panel create` writes the round-N record, and its pass creates the slots. A hand
    `panel create` adopts the authorised round and never skips to N+1. A round is open while one of its slots is open;
    a round whose slots are all closed, `superseded` included, is not open. While a round on the current subject is
    open, `panel create` for that gate refuses and names the open round; redispatch is `panel recover` or `launch`. A
    round on an old subject is superseded by the §13.16 pass before that check. When the subject moves before `panel
    create` opens an authorised round, the pass updates the authorised subject and authorises no new round. On the
    hand path the step is still open: `panel create` closes it first (§13.3), with the branch head at that call as its
    commit (`bead/<w>`, or the spec branch for a final fix), and that close authorises the round on that commit. A
    round past the allowance gives the limit decision, not a `run` entry.
15. **REJECT (OQ13).** A tally with a REJECT verdict is a refused round. Below the limit, its pass creates the fix
    `<gate>.r<N+1>.fix` (B8, B9) and the decision `<gate>.decide.reject.r<N>` with options `fix` and `halt`. While a
    REJECT decision of a gate is open (§5, "an open decision"), a verb that would claim or close another child of that
    gate, open a round on it, or close or merge what the gate decides refuses and names the decision. Examples: `lease`
    or `submit` on the fix, `panel create`, `panel refute`, `panel defer`, `complete`. The fix stays open. A limit
    decision holds only a verb that opens a round or claims or closes a fix. While it is open, `panel refute` and
    `panel defer` run (§13.4); a passing retally closes the gate `passed` and the limit decision `superseded`. This
    difference is intended: a REJECT is a stronger verdict than a limit, and the operator answers it first.
    `next-action` lists the decision, not the fix. `--answer fix` closes the decision and releases the fix; the same
    pass lists the fix as an action. `--answer halt` applies §13.7. The answer adds no round; round N+1, which the fix
    close authorises (§13.14), counts toward `round_limit`. At the limit, the pass creates only the limit decision,
    and `continue:<n>` raises the allowance and creates the fix.
16. **Pass order (OQ12).** Every verb that runs a reducer pass (§4-§7, §13) runs it first, then its own refusal checks
    against the state that pass leaves, then its own write. The pass then resumes after the write: one pass in two
    parts, the one pass of §7, and the second part does the tally and cascades of a close (§6). The first part leaves
    out the kernel step that the verb names by its id (`lease <step>`, `submit <step>`, the slot of `panel submit`).
    After the write, the second part covers that step. When the verb refuses, the writes of its first part stand (they
    are the writes `--apply` makes), it writes nothing on that step, and the next pass covers it. So `submit` with
    valid evidence on an unreceipted close writes the receipt and creates no `<id>.a<k+1>`; after a refused `submit`,
    the next pass creates it (§9). `next-action` without `--apply` and any pass in an unenrolled spec write nothing
    (§1 ruling 1).

## 14. Hand path in an enrolled spec

| Step | Closed by (each verb runs its pass before its checks, §13.16) |
|:-----|:----------|
| implement `<w>.impl.a<k>` | `mindspec next <w>` (no lease token), commit, then `panel create` round 1 |
| round record (round N of a gate) | `panel create` only; orchestrated, it is a `run` entry (§13.14) |
| slot `<w>.review.r<N>.R<s>` | `panel submit` for that slot (`--lease` only on a leased slot) |
| bead gate `<w>.review` | the pass in `panel submit`, `panel refute` or `panel defer` on a passing tally |
| fix `<w>.review.r<N>.fix` | commit on `bead/<w>`, then `panel create` round N; `superseded` if the round passes |
| merge | `mindspec complete <w>` (a `run` entry, never a bead); refusals per §7 and §13.12 |
| final gate `<epic>.final` | the pass in `panel submit` of the final-review panel; a refusal makes its fix |
| final fix `<epic>.final.r<N>.fix` | commit on the spec branch, then the final `panel create` round N |
| decision `<gate>.decide.<cause>.r<N>` | `submit <decision> --answer <opt>` (the one new command) |
