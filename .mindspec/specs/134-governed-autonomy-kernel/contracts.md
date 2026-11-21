# Spec 134 contracts: governed-autonomy kernel

Normative for spec 134. §1-§12 hold the operator rulings of 2026-11-15 and 2026-11-17. §13 holds readings where a
ruling is silent; the operator confirmed them on 2026-11-18, but for those §8 and §13.2 name. The operator rulings of
2026-11-21 hold in B8-B10, §4-§7, §9, §11, §13.2, §13.7, §13.8, §13.10 and §13.11. `§<n>` cites all of it.

## 1. Rulings

1. `next-action` is read-only by default. `--apply` makes steps. Agents probe, so a probe must write nothing.
2. The kernel covers only the plan-approved tree: implement, bead review, fix, decision, final review.
3. The merge stays outside `next-action`: `complete` is a command in the `run` list, never a bead.
4. The whole `loop:` config block is deleted. `Load` warns once per removed key and ignores it.
5. Mindspec has one path. `plan approve` enrols every new spec. There is no hand mode inside mindspec. To work by
   hand means to work without mindspec, in the same bd store (§12).

Thesis: beads hold plan and dispatch; git holds evidence; the binary holds rules. No driver is required.

## 2. Graph model

- Epic and work items `<epic>.<n>` keep their types (`epic`, `task`). Plan order is the approved plan's `depends_on`
  edges (`work_chunks`), which `plan approve` already writes as bd blocks links.
- The kernel alone creates kernel beads, type `step`: implement `<w>.impl.a<k>`; bead gate `<w>.review` and final
  gate `<epic>.final` (both at enrolment); lens `<gate>.r<N>.lens` (when round N is authorised, §13.11); slots
  `<gate>.r<N>.R<s>`; verify `<gate>.r<N>.verify` (after round N is refused, §13.10); fix `<gate>.r<N>.fix` (after
  the verify of round N-1 leaves a reproduced finding); change judge `<gate>.judge.a<k>` (§13.2). Decisions
  `<gate>.decide.<cause>.r<N>` use bd's built-in `decision` type.
- Actor steps close by `submit` or the §7 verb. Gates close only by the reducer on a passing tally. Decisions close
  by `submit --answer`. A gate holds its active round and slot ids in metadata (no round beads, no "pick the
  latest") and is never leased. A slot may close `passed` with a REJECT verdict.
- Each kernel id stores its tuple (epic, work item, gate, kind, round, slot, attempt) in metadata.
- `doctor` reports `step` missing; `plan approve` and `--apply` refuse without it and name `mindspec setup`.

## 3. Rules B1-B13

1. Child = sub-step. A parent closes only when all children are closed. A hold on a parent holds its children.
2. Blocks = downstream waits for upstream to close. A lease on a blocked item or its descendant is refused.
3. Two link kinds only: child and blocks (plus the return kind).
4. The store knows "closed"; the reducer knows "how". The reducer decides what runs. It counts a work item done only
   when its bead gate closed `passed` and `complete` merged its branch; any other close releases no dependent.
5. Every kernel close writes a receipt (metadata `mindspec_close`) with a return kind (`passed`, `refused`,
   `superseded`, `halted`) BEFORE the bd close. A retry with the same receipt completes it.
6. Lineage is metadata `mindspec_from`. Blocks links exist only in plan order and from each work item to
   `<epic>.final` (a `step`, so bd's epic rule does not apply), never to an already-released item.
7. Check both kinds for cycles before each mindspec link write (§8; bd's refusal is the backstop). No blocks between
   ancestor and descendant.
8. Just in time: a bead comes only from the outcome that authorises it (enrolment: all gates; an authorised round:
   its lens; a closed lens: its slots, through `panel create`; a refused round: its verify; a verify that leaves a
   reproduced finding: the fix; a passed fix: the next round; a moved subject at a tally or after a pass: its judge,
   §13.2).
9. A refused tally creates round N's verify under the same open gate (§13.10). A verify that leaves a reproduced
   finding below the limit creates round N+1's fix. Attempts are counted. A verify is not a round.
10. A new subject during round N does not end the round: its slots run to completion on the round's subject, and the
    change judge rules before its tally takes effect (§13.2).
11. `halt` closes each open step under the epic `halted` when it has no open child or blocking upstream.
    A step held by an open foreign child or upstream, and its kernel ancestors, stay open. It never closes a foreign bead.
12. A gate is a child of what it decides. An open bead gate makes raw `bd close <work item>` fail without `--force`.
    A gate closed `passed` is frozen; only a `carry-over` (§13.2) adds to its receipt.
13. A receipt names path, holding commit and blob id, never the body (a bare digest only if a commit keeps the
    bytes). Readers resolve from that commit, not from the branch head.

## 4. `next-action [<spec>] [--apply] [--json]`

Without `--apply` it writes nothing and shows what an apply would do (`"exists": false` for unmade steps). `--apply`
does one reduction under the phase lock: create missing steps, tally inline, cascade, print. It never claims a step
or starts an actor.

```json
{"schema":"mindspec.next-action/v2","status":"work","applied":true,"revision":"sha256:..","retry_at":"<t>",
 "policy":{"round_limit":5,"lease_ttl":"2h"},
 "actions":[{"step":"<w>.review.r2.R3","kind":"review-slot","exists":true,"round":2,"subject":{"commit":"<oid>"},
   "claim":["mindspec","lease","<step>"],"brief":[".."],"submit":[".."]}],
 "waiting":[{"step":"<w>.impl.a1","lease":{"generation":1,"expires":"<t>"}}],
 "decisions":[{"step":"<w>.review.decide.limit.r5","options":["continue:<n>","halt"],"submit":[".."]}],
 "run":[{"actor":"binary","argv":["mindspec","complete","<w>"]}],"flags":[{"code":"unreceipted-close","repair":[".."]}]}
```

- `status` `work` / `wait` / `halted` / `done` / `indeterminate` exits 0 / 3 / 4 / 5 / 1. `indeterminate` means no
  actions. `kind` is `implement`, `lens`, `review-slot`, `verify`, `fix` or `change-judge`; the kind order is that
  list. Commands are argv arrays. Order: plan, kind, slot. Text goes through `termsafe`.
- Inputs: the epic subtree, the git refs it names, the panel ledger and verdicts, the pinned policy, the clock against
  expiries. `revision` digests them (no display times, no free text). Same inputs give the same bytes. An expiry
  changes the output (`retry_at`). A no-change `--apply` writes nothing. A removed field, a new `kind` or a new
  `status` raises `schema`.
- Refuses: no approved plan; a pre-kernel spec (§8); `--apply` with bd silent or no `step`.

## 5. `lease <step> [--actor <id>] [--renew <t> | --release <t> | --take --reason <text>]`

bd's lease cannot serve: its holder is the bd actor (one per clone) and its TTL is in minutes. Under the phase lock,
`lease` writes `in_progress`, a monotonic `generation`, the expiry and a token hash with bd `--if-status`
compare-and-set. It prints the token once. `lease <w>.impl.a<k>` and `next <w>` are the same claim. Refuses: a foreign
bead, a gate, a decision; a held or blocked ancestor or upstream (B1, B2; an open epic, work item or gate does not
hold); a live lease; a halted spec; an unanswered decision (§13.8); a review slot, verify or fix with no `--actor`;
an actor that §13.10 excludes. `--actor` names the holder, a label the orchestrator assigns; the lease record stores it
(`actor`). `--take` revokes, issues generation N+1 and appends a take event to `mindspec_takes`: reason, time,
generation, actor, the revoked generation's token hash (`revoked_token_hash`) and the step's subject at the take
(`subject`). `panel.lease_ttl` defaults to `2h` and is renewable.
The metadata key `mindspec_raw_claims` holds an array of raw-claim records: `[{"at":"<RFC3339 time>"}]`.
Each raw-claim record stores the time when a lease replaces a raw claim. The lease write appends the record and keeps earlier records.

The work-item metadata key `mindspec_completed_subject` holds a commit id as a JSON string. `complete` writes it
after the item is closed and merged and its halt, blocker, and gate checks pass. It must equal the accepted subject
for the item to count as done.
An epic, work item or gate with status `in_progress` does not hold work. The pass flags `raw-claim` and names the record.

## 6. `submit <step> --lease <t> (--artifact <path> | --commit <oid>)` | `submit <step> --take --reason <text> (--artifact <path> | --commit <oid>)` | `submit <decision> --answer <opt>`

Checks: the generation; subject = the gate's active round; the artifact resolves at a commit and the blob matches; the
artifact fits the kind (a lens record for a lens step, §13.11; a verdict for a slot; a verify record for a verify step,
§13.10; a commit on `bead/<id>` past the round subject for a fix; a commit past the claim base for an implement step; a
judgement for a change judge, §13.2). It writes the receipt (with the accepted `generation` and the holder's `actor`),
closes, and runs one reducer pass (order: §13.9). Refuses: an absent token; a token of no generation of the step; a
stale subject; a foreign slot; a missing artifact; a different result for a closed step. **Late submission:** a token
of a revoked generation (it matches a take event's `revoked_token_hash`) is accepted when the step is open, its subject
equals that take event's `subject`, and no newer generation has submitted. The current lease then ends `superseded`
(lease record `ended`), and the receipt names the accepted generation and the superseded one
(`superseded_generation`). Else it refuses and names the condition. `--take` (§9) needs no token. The same receipt again on a closed step exits 0 with no change; on an
open step it closes it. A decision needs no token and takes a listed option; it counts from its receipt, even when a
foreign child keeps it open.

## 7. Existing verbs

The final `impl approve` run entry appears only when every work item is done, the active final gate has passed,
no child, judge, or blocker is open, the epic is open, and the spec is not halted.

The existing verbs stay. `run` lists them; each closes the kernel step it names, runs one reducer pass (order:
§13.9) and keeps today's refusals. `plan approve` enrols the spec (§8) and runs no reducer pass.

- `next <w>`: takes the lease of `<w>.impl.a<k>` exactly as `lease` does and prints the token. With a live lease it
  refuses and names `lease --take`. It selects only approved work items, by computed id.
- `panel create`: the one writer of a round record (§13.7); takes the subject from the gate; renders the BRIEF body
  (§13.11); refuses another `--target`, a round past the allowance, an open round lens step, and `--brief-file`.
- `panel submit`: closes the matching slot as `submit` does; it needs `--lease` of the current generation.
- `panel refute` / `panel defer`: a passing retally closes the gate `passed` and its open verify or fix
  `superseded`.
- `brief <step>`: prints the brief of an actor step of an enrolled spec (§13.11) and writes nothing. It refuses
  another step.
- `complete`: refuses unless the bead gate closed `passed` and no child is open. On a raw-closed, unmerged work item
  whose gate closed `passed`, it merges and clears `raw-close`. For another closed, unmerged item or `forced-close`, it
  refuses with the recovery line: `bd reopen <w>`, then finish the gate. A rerun on a merged item exits 0, no change.
- `impl approve`: prints the audit table and runs today's finalize export; refuses on an open step, an open
  decision, an unrepaired unreceipted close, `raw-close` or `forced-close`.
- `config show` / `Load`: one truthful label per key; one warning per removed key. There is no `decide`, `halt`,
  `adopt`, `takeover`, `mode` or `run start` verb.

## 8. Enrolment

`plan approve` enrols every new spec. Under the phase lock it writes in this order: each work item, with its id recorded
in the plan's `bead_ids` right after its create; epic metadata `mindspec_links` (each recorded id with its plan-order
upstream ids; each run writes it whole here); a B7 cycle check of its links against the store; the plan-order blocks
links; epic metadata `mindspec_run: {round_limit, lease_ttl, rules: "kernel-1"}`; the approval record; the gates, with
the work-item links to `<epic>.final` (B5: `--apply` creates a missing one). The plan is unapproved until the approval
record. If the check or bd refuses a link as a cycle before `mindspec_run`, `plan approve` refuses and names the link
and any foreign link that closes the cycle, with its bd removal; it writes no `mindspec_run`, approval record or gate.
From `mindspec_run` on, the spec is enrolled. A link that the check or bd refuses after that (here or at `--apply`) is
not written: the step that needs it stays open, `plan approve` exits 0, and the next `--apply` tries again. Each pass
runs the B7 check on current state for each missing mindspec link and flags `foreign-blocker` naming the first foreign
link on each cycle from that link's downstream bead, and its bd removal; nothing is stored. A rerun before the approval
record keeps any `mindspec_run`, reuses exactly the recorded ids, creates only chunks with no recorded id and continues
the order. It compares first: each id in a written `mindspec_links` against the plan. An id not in it (created by a
killed run, so it has no link yet) is a missing item and takes its links from the plan, edited or not (applied
reading). With no recorded id in the store (a foreign child does not count), it ignores the recorded ids and a
written `mindspec_links`. It never selects by `--parent` and closes or supersedes no other bead. It refuses every other
plan change, names it and writes nothing: a recorded id with no chunk, a recorded chunk whose text differs from its
item, or an id in a written `mindspec_links` whose plan-order upstreams differ from it (it adds no link between existing
items and keeps no dropped plan-order link). On an enrolled spec with an approval record, `plan approve` refuses, names
the enrolment and writes nothing. An item created but not recorded is foreign; its `foreign-child-open` recovery line
says to close it with bd. A spec is enrolled if ANY witness exists: `mindspec_run` on the epic; a computed kernel id
under the epic (open or closed, any type); a spec `panel.json` with `"orchestrated": true` (`panel create` writes it). A
witness without the marker gives the default policy and flags `run-marker-missing`. A spec with no witness (approved
before this release) is pre-kernel and finishes on today's rules: warnings, and `panel_presence` applies. `next-action`
and `lease` refuse it, naming today's verbs; no pass writes on it. No verb, flag, env var or key enrols a pre-kernel
spec or clears enrolment. Policy is pinned at enrolment; config edits and retypes change nothing. An enrolled spec
always enforces the panel gate. Threat model: the kernel governs progression, not OS access; a same-user process can
forge an answer. One controller host.

## 9. Refused vs flagged

In an enrolled spec, the verbs refuse what §7 lists. Flagged: a raw close of a kernel step (not evidence); a raw close
of a work item (`raw-close`) and a `--force` close over an open child (`forced-close`), each with a §7 `complete`
recovery; a raw claim (§12 item 3). Flags are computed from current state, never stored. Counted, never blocked:
escape hatches, deferrals, `--take`, decision answers. `MINDSPEC_SKIP_PANEL=1` does not open or pass a gate: with no
passing gate the verb refuses and names it; with a passing gate it is ignored and the verb proceeds (pre-kernel: as
today). Floor: `FindOrphanedClosedBeads` and the self-orphan handling in `complete`. **Unreceipted close** (no or bad
receipt): with evidence and a generation §6 accepts, `submit` writes the receipt (no reopen). Else, when the step's
evidence resolves at a commit and fits its kind and current subject (a slot's verdict at its registered path; an
implement or fix commit past its claim base or round subject on the branch the gate names; a lens or verify record at
its kernel path; a judgement in a registered panel directory whose new subject is the judge's subject), the pass lists
`submit <id> --take --reason <text> (--artifact | --commit) <evidence>` in `run` and creates no retry. That submit
needs no token; it appends a take event (`via: "receipt"`) and writes the receipt (`repaired: true`, `taken: true`).
Only when no evidence resolves and fits does the next pass create `<id>.a<k+1>` with `mindspec_from`.

## 10. Rounds and decisions

`panel.max_rounds` defaults to 5. `loop.halt.max_rounds_per_bead` is a warned alias for one release, read only when
`panel.max_rounds` is unset; the warning names the value. `panel_deadlock_rounds` and `max_consecutive_impl_failures`
are deleted. Rounds are counted per gate over every panel registration of the work item, abandoned included, from the
git ledger (not per directory). Redispatch (`panel recover` / `panel launch`) is free. At the limit: a decision
`continue:<n>` (raises the allowance; history kept) or `halt`. A REJECT gives a decision for that item only: `fix`
(counted) or `halt`. There is no `accept`: findings are fixed, refuted or deferred. `halt` closes open steps and
leaves the epic and work items open (§13.4). Spec and plan gates: count and warn.

## 11. Audit table

`impl approve` prints the table to stderr; the table never blocks approval. It shows policy and counts of each escape hatch (`--allow-doc-skew`,
`--override-adr`, `--allow-not-ready`, `--allow-net-deletion`), deferrals, takeovers, timed decision answers, raw
claims, unreceipted closes, carry-overs, the slot count of each re-panel, verify returns by type (`reproduced`,
`refuted`, `deferred`), late submissions accepted (receipts with `superseded_generation`, §6), receipts written under
`submit --take` (`taken: true`, §9) and lens steps per gate round, from durable records. `--allow-not-ready` joins
both friction registries.

Takeover history is stored in `mindspec_takes`, with one event per lease generation.
Deferral history is stored in `mindspec_deferrals`, with distinct panel, round, slot, finding and target records.
The latest `mindspec_take` and `mindspec_defer` records remain readable.

## 12. Shared bd store

A person or agent that does not use mindspec (a "foreign user") works in the same bd store with bd and git only,
with no mindspec command and no setup. Mindspec-owned = the computed set: the epic, the approved work items (the
finalizer's allow-set) and kernel ids. Every other bead is foreign, a `task` under the epic included. Nothing a
foreign user does with bd makes mindspec advance, lose, double or corrupt a step.

1. **Ids.** Kernel ids have word segments (`.impl.`, `.review`, `.final`, `.lens`, `.verify`, `.decide.`, `.judge.`). bd assigns
   `<prefix>-<hash>` and numeric child ids `<parent>.<n>`, so a bd-assigned id never has a kernel shape. Work items are
   the approved plan's ids, never "any `task` child": a foreign child `<epic>.<n>` stays foreign, and phase derivation
   and `next` selection ignore it. A bead that has a kernel-shaped id but no matching tuple (for example from `bd create
   --id`) is a collision: the verb refuses, prints `id-collision` with the id and the line "remove that bead with bd",
   and never adopts, edits or closes it. Kernel-shaped = an exact match of the §2 grammar, with an optional attempt
   suffix `.a<k>` on any kernel id: a bd-numbered child such as `<w>.review.1` is foreign. If OQ1 takes its fallback,
   the computed key in metadata is the identity.
2. **Views.** bd 1.3.4 has no store-level default filter for `bd ready` or `bd list` (`bd config --help` lists
   none). Every bead mindspec creates (epic, work items, steps, decisions) gets label `mindspec` and a title that
   starts `[mindspec]`; types stay as §2 says. `bd ready --exclude-label mindspec` hides them all. (OQ14.) The epic
   exists from `spec approve`; `plan approve` adds the label and prefix to it when they are missing. The spec-id parse
   of a title skips the `[mindspec]` tag.
3. **Claims and closes.** bd 1.3.4 accepts `--claim` on any bead, blocked ones included. A foreign claim of a mindspec
   bead is not a lease and never advances mindspec: the pass flags `raw-claim`, and `lease` takes the step with
   compare-and-set on the status it read. A foreign close is not evidence (§9). bd's open-child refusal stays (B12).
4. **Links.** Mindspec writes links only between computed ids. A foreign link to or from a mindspec bead is respected
   as a blocker and never removed. A cycle at a mindspec link write: §8. A foreign link or blocker that holds a
   mindspec step: the pass flags `foreign-blocker` naming it, the step waits, and the recovery line says to remove
   that link or close that blocker with bd.
5. **Writes.** Mindspec never creates, edits, claims, closes, retypes, relabels, reparents or deletes a foreign bead.
   When an open foreign child stops a mindspec close (gate, slot, fix, `complete`, `impl approve`), the close does not
   happen: the verb checks before any write, the pass flags `foreign-child-open` naming the child, and the recovery
   line says to close or re-parent that child. The flag shows whenever an open foreign child is under a mindspec bead;
   status is `wait` only when no action remains. Every query that feeds a write, and every bulk write (`halt`, any
   supersede, the finalize allow-set), selects by computed or recorded id, never by type, label, parent or prefix. A
   deleted kernel bead is created again; its rounds stay in the ledger.
6. **Types and config.** `mindspec setup` adds `step` to `types.custom` and keeps every value already there. It
   replaces no value of a key outside mindspec's required set, `--force` included.
7. **Concurrent writes.** Every kernel status write uses bd `--if-status` compare-and-set. On a mismatch (bd exit 13,
   nothing written) the verb reads again and runs its pass once more. A retry that succeeds returns success. A second
   mismatch refuses; the step stays open with at most its receipt (B5). Creates use computed ids: no second bead.

## 13. Applied readings (confirmed by the operator, 2026-11-18, but for those §13.2 names)

1. **Final gate (OQ4).** B9, the verify step (§13.10) and the decision grammar apply to `<epic>.final`. A final fix commit lands on the spec
   branch, which the §6 fix check reads. Final rounds count over all `final_review` registrations, abandoned too.
2. **Change judge (OQ5).** A move of a gate's subject commit never by itself forces a new panel or ends a round, and
   neither a frozen receipt nor a tally by itself authorises the new subject. The accepted subject of a passed gate is
   the later, in pass order, of the passing panel's subject as a successful `panel rebind` refreshes it (rebind does
   not write the gate; it runs one pass) and the new subject of the last `carry-over`. The accepted subject of an open
   gate is the subject of its last registered round (for a successor with none, its frozen predecessor's accepted
   subject), or the new subject of a later `carry-over`. A commit that changes only a panel directory of any
   registration of the gate, or the kernel directory (§13.11), does not move the subject. A pass creates the actor step
   `<gate>.judge.a<k>` (`<gate>` is the base id `<w>.review` or `<epic>.final` for every attempt; kind `change-judge`,
   child of the gate's parent, subject = the newest subject commit), leased and submitted as §5 and §6 say, when no
   judge is open on that commit and the newest subject commit is not the accepted subject, in each of these cases:
   (a) a tally or a verify retally (§13.10) of an open gate is due; the outcome waits for the judgement; (b) the gate
   is passed (highest attempt not closed `superseded`), the work item of a bead gate is not merged and `impl approve`
   has not closed the epic; (c) a `re-panel` round is authorised and not registered (§13.7). One subject change has
   one live judge: a pass closes an open judge `superseded` when its subject is not the newest commit or is the
   accepted subject (a successful rebind; the receipt names the rebind), then applies the rule above, so any `.a<k+1>`
   judges the newest commit. The artifact, in a registered panel directory of the gate like a verdict, is a judgement
   with one of two values: `carry-over` with a reason, or `re-panel` with a slot count n and the changed paths. n is
   from 2 to the slot count of the gate's passing round, or for an open gate of the round its tally covers; after a
   1-slot round only `carry-over` fits. `submit` checks the shape, the old subject (the accepted subject), the new
   subject and, for a passed gate, the frozen gate's receipt. It computes the changed paths, `git diff --name-only`
   from the old to the new subject without the panel directories and the kernel directory, and refuses a `re-panel`
   whose paths do not include each of them, naming each missing path. It never judges the content. A judgement that
   does not fit is refused, step open. Outcomes:
   - Open gate, `carry-over`: the gate round record adds the judge and the new subject (`carry_over`), and the waiting
     outcome applies to the new subject: a passing tally closes the gate `passed`; a refused tally creates the verify.
   - Open gate, `re-panel`: the gate takes the re-panel record. A refused tally creates the verify on the new subject.
     A passing tally, or a verify that leaves no reproduced finding, authorises round N+1 of n slots on the new subject
     on the same gate, with the paths as its scope. A verify that leaves a reproduced finding creates the fix, and the
     round after the fix is a full round.
   - Passed gate, `carry-over`: the gate stays `passed`; its receipt adds the old subject, the new subject and the
     judge's receipt, and verbs accept the new subject.
   - Passed gate, `re-panel`: the pass creates the successor `<gate>.a<k+1>` (same parent, `mindspec_from` = the
     frozen gate, the re-panel record) and authorises a round of n slots on the new subject; its `panel create` entry
     in `run` (§13.7) carries the changed paths as the round's scope.
   Verbs select the highest attempt not closed `superseded`, rounds keep counting, and §2 ids apply to it. Judge and
   gate attempts count apart. An `impl approve` killed after its finalize export commit, before the epic closes, leaves
   a final judge that `carry-over` clears. Applied readings, not in the ruling: the accepted subject and its
   precedence, rebind's pass, the panel-directory exemption, the end state, judge parent and id, supersede of a judge,
   separate attempt counters, slot ceiling, scope carrier.
3. **Implement authorisation (OQ9).** An open work item with all plan upstreams done (B4), no open implement step and
   no gate round authorises `<w>.impl.a1`. Each pass creates it for every such item, in plan order, on the one
   controller host (§8); this is not the deferred parallel waves. `<w>.impl.a<k+1>` comes only from §9.
4. **After halt (OQ6).** The spec stays enrolled. `halt` closes every open step and decision under the epic `halted`
   that it can. A step with an open foreign child or blocking upstream, and its kernel ancestors, stay open.
   The pass flags the foreign child or blocker. It writes no halt receipt on a step that cannot close.
   The spec is halted from the answer's receipt. Then only writes that finish the halt run: a retry of the same answer
   and `next-action --apply` each close the receipted decision and every other closable step `halted`, exit 4 (not
   §6's 0) and write nothing else. Any other verb that would create, claim or close a kernel step, or claim, close or
   merge a work item or the epic, refuses, names the halt and writes nothing (no pass). A halted spec is not active
   when a verb resolves its spec without `--spec`. Read verbs work: `next-action` (prints `halted`, exits 4, writes
   nothing; `halted` wins over `wait`), `panel tally`, `panel verify`, `config show`, `doctor`. Nothing resumes a
   halted epic. Each `halt` option says it stops every work item.
5. **Refusal sets (OQ11).** No refusal list in §4-§9 or `spec.md` is exhaustive. Each verb keeps today's refusals
   and adds every refusal that §5-§12 give it. Flag codes: `unreceipted-close`, `raw-close`, `raw-claim`,
   `forced-close`, `run-marker-missing`, `foreign-child-open`, `foreign-blocker`, `id-collision`.
   - `open-step`: an actor step remains open when implementation approval checks the epic.
   - `open-decision`: a decision remains open when implementation approval checks the epic.
   - `work-item-blocked`: a work item has a hold or blocker that prevents implementation approval.
6. **Skill halt rows (OQ11).** A REJECT is local to its gate. Rows in both copies of each skill that halt the whole
   spec go: in `ms-bead-cycle`, "any REJECT / HARD block → halt", "do not proceed to the next bead", `max-rounds`; in
   `ms-spec-autopilot`, "One halted bead halts the spec", "Any bead REJECT verdict", "Two consecutive impl-subagent
   failures", `max-rounds-per-bead`, the NOT READY row, "Don't keep going after a halt". Rows that stay: a failed
   merge, a dependency mismatch, and rows that return an open decision to the user.
7. **Round opening (OQ10, OQ12).** `panel create` is the one writer of a round record (the registration and its ledger
   `round` line, which `ForBead` and §10 count). These outcomes authorise round N on a subject commit: a closed
   implement step (round 1); a closed fix `<gate>.r<N>.fix` (round N); a `re-panel` judgement, or under it a verify
   that leaves no reproduced finding (§13.2: n slots, on the successor of a passed gate or on the open gate); for
   `<epic>.final`, the last work item done (B4; round 1, the spec branch head). The pass records the round and subject
   in gate metadata and creates the lens step `<gate>.r<N>.lens` (§13.11), for a scoped round too. When the lens step
   is closed `passed`, the pass lists `panel create` for that gate in `run`; its pass creates the slots. While a round
   on the current subject has an open slot, `panel create` refuses and names it (redispatch: `panel recover` or
   `launch`). A subject move before `panel create` updates the authorised subject only and keeps the lens step, except
   after a `re-panel` judgement: a later non-panel commit before `panel create` opens a new judge on the newest subject
   (§13.2 case c), which rules on the whole diff from the accepted subject. Its outcome replaces the scoped round: the
   pass closes the unregistered successor `superseded` (on an open gate, it withdraws the authorised round) and its open
   lens step `superseded`, then applies §13.2 to the frozen or open gate. A panel-only commit does not move the
   subject. The re-panel record states what the judge authorised; the gate round record alone states the slots that
   run. A round past the allowance gives the limit decision, not a lens step or `run` entry.
8. **REJECT (OQ13).** A tally with a REJECT verdict is a refused round, so it creates the verify (§13.10). Below the
   limit, a verify that reproduces a REJECT finding creates the fix `<gate>.r<N+1>.fix` and the decision
   `<gate>.decide.reject.r<N>` (`fix` or `halt`); a refuted or deferred REJECT finding gives no decision. While the
   decision is unanswered, a verb that would claim or close another child of that gate, open a round on it, or close or
   merge what it decides refuses and names it; `next-action` lists the decision, not the fix, until `--answer fix`
   releases the fix. A limit decision holds only a verb that opens a round or claims or closes a fix; a passing `panel
   refute` or `panel defer` retally closes the gate `passed` and the limit decision `superseded`. The answer adds no
   round. At the limit, a verify that leaves a reproduced finding creates only the limit decision, and `continue:<n>`
   raises the allowance and creates the fix.
9. **Pass order (OQ12).** A verb runs its pass first (leaving out the step it names), then its refusal checks, then
   its own write, then the rest of the pass. On a refusal the first part stands and the next pass covers the step.
10. **Verify.** A refused tally, after any judgement (§13.2), creates the actor step `<gate>.r<N>.verify` (kind
    `verify`, child of the gate, subject = the round subject or the judged new subject), leased and submitted as §5 and
    §6 say. It runs at every gate B9 covers, `<epic>.final` included, below and at the limit. A verify is not a round
    and adds none. The artifact `<spec dir>/kernel/<step id>.json` is a verify record: `{"step", "subject", "returns":
    [{"slot", "round", "finding", "finding_sha256", "return", "evidence", "target"}]}`. `finding` is `ccr[<i>]`, or
    `slot` for a verdict that is not APPROVE and has no `concrete_changes_required`, as `panel refute` names it.
    `submit` checks shape only: the returns cover each finding of round N's receipted verdicts exactly once, by slot,
    round, finding and `finding_sha256`; `return` is `reproduced`, `refuted` or `deferred`; a `refuted` return has a
    non-empty `evidence`; a `deferred` return has a non-empty `target`. It refuses and names each missing, repeated or
    unknown finding, each unlisted return and each missing evidence or target. On the close it records each `refuted`
    return as `panel refute` records a refutation and each `deferred` return as `panel defer` records a deferral
    (`mindspec_deferrals` included), each naming the verify step. The retally reads the verify receipt. With no
    `reproduced` return, the gate closes `passed` (receipt `retally: "verify"`), a REJECT verdict included, and no fix
    is created (under a `re-panel` judgement: §13.2). Else B9 and §13.8 apply, and the fix's metadata
    `mindspec_reproduced` lists the reproduced findings (slot, round, finding, `finding_sha256`) as its scope. A passing
    `panel refute` or `panel defer` retally closes an open verify `superseded`. Independence is a shape check on the
    declared actors (§5): `lease` of a verify refuses an actor recorded on any slot of round N (lease, take event or
    receipt), and `lease` of the fix `<gate>.r<N+1>.fix` refuses the actor recorded on the verify's receipt. Each
    refusal names the step that holds the actor. The binary compares labels and does not authenticate them (§8, threat
    model). An approval count never resolves a finding: each finding is fixed, refuted or deferred.
11. **Briefs and lenses.** The binary renders every brief of an enrolled spec from durable records; no actor writes a
    brief. The `brief` argv of each action: implement, `next <w> --emit-only` (work item, spec, plan); review slot,
    `panel prompt` (the managed header, then a generated body: a summary from the work item; the files in scope, from
    the round paths, else `git diff --name-only` from the claim base (round 1) or the prior round's subject; the prior
    round's asks from its verdicts, each with its verify return and evidence or target; the slot's lens); lens, verify,
    fix and change judge, `brief <step>`. A lens brief gives the spec, plan, work item and the diff under review; a
    verify brief gives round N's findings (slot, finding, text) and the subject; a fix brief gives the findings in
    `mindspec_reproduced`, the subject and the work-item context; a change-judge brief gives the accepted subject, the
    new subject, the diff between them and the slot ceiling (§13.2). **Lens step.** When a round is authorised, the
    pass creates `<gate>.r<N>.lens` (kind `lens`, child of the gate, subject = the authorised subject) before the
    `panel create` entry, for a scoped round too. The artifact `<spec dir>/kernel/<step id>.json` is a lens record:
    `{"step", "lenses": [{"slot": "R<s>", "lens": "<text>"}]}`. `submit` checks shape only: exactly one non-empty lens
    for each slot of the round's roster except the adversary slot (the slot whose configured lens is `adversarial`),
    and none for it; it refuses and names each missing, extra or empty entry. `panel create` refuses while the round's
    lens step is not closed `passed` and names it; it records each lens in the gate round record (`mindspec_round`
    `lenses`) and as the slot's `focus` in `panel.json`. Coverage floor: each slot keeps the managed header (the spec's
    acceptance criteria and the acceptance question); a lens adds focus and replaces neither. The adversary slot always
    runs its configured lens. The kernel directory `<spec dir>/kernel/` holds lens and verify records; the finalize
    export treats it as a panel directory. A pre-kernel spec, and the spec and plan gates, keep the BRIEF stub and
    `--brief-file`.
