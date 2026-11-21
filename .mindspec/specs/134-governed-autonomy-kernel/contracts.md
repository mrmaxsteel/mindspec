# Spec 134 contracts: governed-autonomy kernel

Normative for spec 134. §1-§12 hold the operator rulings of 2026-11-15 and 2026-11-17. §13 holds readings where a
ruling is silent; the operator confirmed them on 2026-11-18, but for those §8 and §13.2 name. The operator rulings of
2026-11-21 hold in B8-B10, §4-§7, §9, §11, §13.2, §13.7, §13.8, §13.10 and §13.11, but for the readings that the
§13.2 list "Applied readings, not in the ruling" names. `§<n>` cites all of it.

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
  gate `<epic>.final` (both at enrolment); lens `<gate>.r<N>.lens` (when round N is authorised, §13.11; attempt
  `.a<k+1>` on each later authorisation or subject move, §13.7); slots
  `<gate>.r<N>.R<s>` (attempt `.a<k+1>` on a redispatch, §13.10, or a retry, §9); verify `<gate>.r<N>.verify` (after
  a complete round N is refused; attempt `.a<k+1>` when the pass retires the current one, §13.10); fix `<gate>.r<N>.fix` (after
  the verify of round N-1 leaves a reproduced finding); change judge `<gate>.judge.a<k>` (§13.2). Decisions
  `<gate>.decide.<cause>.r<N>` (cause `reject`, `limit` or `uncounted`; attempt `.a<k+1>` for a later `uncounted`
  or `limit` decision of round N, §13.8, §13.10) use bd's built-in `decision` type.
- Actor steps close by `submit` or the §7 verb. Gates close only by the reducer on a passing tally or verify
  (§13.10). Decisions close by `submit --answer`. A gate holds its active round and slot ids in metadata (no round beads, no "pick the
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
   its lens; a closed lens: its slots, through `panel create`; a complete, refused round: its verify; a slot verdict
   that the tally does not count: that slot's next attempt, or at the cap the `uncounted` decision and, on its `redispatch`, the next attempt of each listed slot still uncounted (§13.10); a verify that leaves a
   reproduced finding: the fix; a passed fix: the next round; a moved subject at a tally or after a pass: its judge,
   §13.2).
9. A complete, refused tally creates round N's verify under the same open gate (§13.10). A verify that leaves a reproduced
   finding below the limit creates round N+1's fix. Attempts are counted. A verify is not a round.
10. A new subject during round N does not end the round: its slots run to completion on the round's subject, and the
    change judge rules before its tally takes effect (§13.2; a frozen round's retally: §13.10).
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
   "claim":["mindspec","lease","<step>","--actor","<actor>"],"brief":[".."],"submit":[".."]}],
 "waiting":[{"step":"<w>.impl.a1","lease":{"generation":1,"expires":"<t>"}}],
 "decisions":[{"step":"<w>.review.decide.limit.r5","options":["continue:<n>","halt"],"submit":[".."]}],
 "run":[{"actor":"binary","argv":["mindspec","complete","<w>"]}],"flags":[{"code":"unreceipted-close","repair":[".."]}]}
```

- `status` `work` / `wait` / `halted` / `done` / `indeterminate` exits 0 / 3 / 4 / 5 / 1. `indeterminate` means no
  actions. `kind` is `implement`, `lens`, `review-slot`, `verify`, `fix` or `change-judge`; the kind order is that
  list, then `decision` (the `decisions` array). Commands are argv arrays; `<actor>` and `<text>` are placeholders
  that the orchestrator fills, in `actions` and in `run` entries. Order: plan, kind, slot. Text goes through `termsafe`.
- Inputs: the epic subtree, the git refs it names, the panel ledger and verdicts, the pinned policy, the clock against
  expiries. `revision` digests them (no display times, no free text). Same inputs give the same bytes. An expiry
  changes the output (`retry_at`). A no-change `--apply` writes nothing. A removed field, a new `kind` or a new
  `status` raises `schema`.
- Refuses: no approved plan; a pre-kernel spec (§8); `--apply` with bd silent or no `step`.

## 5. `lease <step> --actor <id> [--renew <t> | --release <t> | --take --reason <text>]`

bd's lease cannot serve: its holder is the bd actor (one per clone) and its TTL is in minutes. Under the phase lock,
`lease` writes `in_progress`, a monotonic `generation`, the expiry and a token hash with bd `--if-status`
compare-and-set. It prints the token once. `lease <w>.impl.a<k> --actor <id>` and `next <w> --actor <id>` are the same
claim. Refuses: a foreign bead, a gate, a decision; a held or blocked ancestor or upstream (B1, B2; an open epic, work
item or gate does not hold); a live lease; a halted spec; an unanswered decision (§13.8); a new generation with no
`--actor`; an actor that §13.10 excludes. `--actor` names the holder, a label the orchestrator assigns; `--renew` and
`--release` keep it. Each new generation (first lease, replacement of an expired lease, or `--take`) stores the actor in
the lease record (`actor`) and appends `{"generation", "actor", "via"}` to `mindspec_actors` (`via`: `lease`,
`take`, or `receipt` for `submit --take`, §9); earlier entries stay. `--take` revokes, issues generation N+1 and appends a take event to `mindspec_takes`:
`via: "lease"`, reason, time, the new generation, the new holder's `actor`, `revoked_actor`, the revoked generation's
token hash (`revoked_token_hash`) and the step's subject at the take (`subject`, §6). `panel.lease_ttl` defaults to
`2h` and is renewable.
The metadata key `mindspec_raw_claims` holds an array of raw-claim records: `[{"at":"<RFC3339 time>"}]`.
Each raw-claim record stores the time when a lease replaces a raw claim. The lease write appends the record and keeps earlier records.

The work-item metadata key `mindspec_completed_subject` holds a commit id as a JSON string. `complete` writes it
after the item is closed and merged and its halt, blocker, and gate checks pass. It must equal the accepted subject
for the item to count as done.
An epic, work item or gate with status `in_progress` does not hold work. The pass flags `raw-claim` and names the record.

## 6. `submit <step> --lease <t> (--artifact <path> | --commit <oid>)` | `submit <step> --take --actor <id> --reason <text> (--artifact <path> | --commit <oid>)` | `submit <decision> --answer <opt>`

Checks: the generation; the step's subject (**Subjects**); the artifact resolves at a commit and the blob matches; the
artifact fits the kind (a lens record for a lens step, §13.11; a verdict for a slot; a verify record for a verify step,
§13.10; for a fix, a commit on the branch the gate names past the fix step's subject that changes, relative to that
subject, a path outside the panel directories and the kernel directory, §13.11, so the step's subject itself and any
commit that the step's subject contains are refused; a commit past the claim base for an implement step; a judgement
for a change judge, §13.2). For a lens or verify step it first makes the close writes of its kind (§13.10, §13.11).
Then it writes the receipt (with the accepted `generation` and that generation's `actor` from
`mindspec_actors`), closes, and runs one reducer pass (order: §13.9). The §9 receipt with no reopen and `submit --take`
make the same close writes before their receipt. **Closing pin** (verify only). On every receipt path the pin runs
after every other check of this section passes, the token and generation included, so a refused token, a foreign slot
or a bad record writes nothing. On a step with a valid receipt the closed-step rules below apply and the pin writes
nothing. On a step with no valid receipt, the submit builds and size-checks every close write (§13.10 bound) and then,
when absent, writes the step metadata `mindspec_closing` {`generation`, `actor`, `record_sha256` (the sha256 of the
record bytes at the resolved commit), `at` (RFC3339Nano UTC from `kernel.Now`)}. A submit whose generation, actor or
`record_sha256` differs refuses, names `mindspec_closing`, and writes only `mindspec_closing.conflict` (its
generation, actor and `record_sha256`). A take after a pin thus retires the attempt also for the same record bytes,
as the ledger records name the pinning actor. While `conflict` is set, each submit of that attempt refuses and writes
nothing, and `submit` reads `conflict` again just before the receipt write. The next pass retires the current attempt
when it has no valid receipt and its `mindspec_closing` has a `conflict`: it closes it `superseded` when open and
creates `<id>.a<k+1>` with the keys of its kind (§9, §13.10). A matching rerun (same generation, actor and
`record_sha256`, no `conflict`) reuses `mindspec_closing.at` and makes each close write again from the same record:
it writes `mindspec_returns` and the `mindspec_deferrals` entries of this `verify_step` by key, and appends the same
ledger records (same key, same content, `created_at` = `mindspec_closing.at`), which the ledger keeps once; then it
writes the receipt. A lens has no pin: its one close write, `mindspec_round.lenses`, is replaced whole on each run,
so the lenses that stand are those of the submit that wrote the receipt (§13.11). Refuses: an absent token; a token of no
generation of the step; a stale subject; a foreign slot; a missing artifact; a different result for a closed step (the
step stays closed and its receipt does not change); a `submit --take` on a step that is open, on a step that was never leased, on a step
with a valid receipt, or with evidence that does not resolve or fit its kind and subject (§9), each with the step unchanged.
**Generation rule** (`submit` and `panel submit`, §7): a token of
the current generation is accepted. A token of a revoked generation (it matches a take event's `revoked_token_hash`) is
accepted when the step is open and no newer generation has a receipt or (verify) a `mindspec_closing`. An open step keeps the take event's `subject`,
because a step's subject never changes: a subject move closes an open lens or judge step `superseded`, and the pass
closes an open verify `superseded` when it retires it, each with a new attempt (§13.2, §13.7, §13.10). On a late acceptance the current lease ends `superseded` (lease
record `ended`), and the receipt names the accepted generation, its actor and the superseded generation
(`superseded_generation`). Else it refuses and names the condition. The same receipt
again on a closed step exits 0 with no change (its close writes came before the receipt); on an open step it closes it. A decision needs no token and takes a
listed option; it counts from its receipt, even when a foreign child keeps it open.

**Subjects.** The round subject is `mindspec_round.subject`; registration fixes it, and a commit during the round
does not change it (B10). The review subject of an open gate is the new subject of the latest judgement on its round
(`mindspec_round.judged`, §13.2) when one is recorded, else the round subject. Each step has one subject, set when the
pass creates it (`mindspec_subject`): a slot, the round subject; a verify or a fix, the review subject (§13.10 retires a
verify whose subject a later judgement replaces); a lens step, the authorised subject (§13.7 gives a fresh attempt when
it moves); a change judge, the newest subject commit; an implement step, its claim base. `activeActorRound`
(`submit.go`) compares a slot with the round subject, a verify or a fix with the review subject, and a lens with the
authorised subject; `checkFix` measures from the fix step's subject. Panel evidence (registration, ledger, verdict
`reviewed_sha`) still matches the round subject. `gatePassEvidence`, `passed()`, `tally` and `AcceptedSubject`
return the review subject as the accepted subject of an open gate, and a gate closes `passed` with the review subject
in its receipt.

## 7. Existing verbs

The final `impl approve` run entry appears only when every work item is done, the active final gate has passed,
no child, judge, or blocker is open, the epic is open, and the spec is not halted.

The existing verbs stay. `run` lists them; each closes the kernel step it names, runs one reducer pass (order:
§13.9) and keeps today's refusals. `plan approve` enrols the spec (§8) and runs no reducer pass.

- `next <w> --actor <id>`: takes the lease of `<w>.impl.a<k>` exactly as `lease` does and prints the token. With a live
  lease it refuses and names `lease --take`. It selects only approved work items, by computed id.
- `panel create`: the one writer of a round registration (§13.7). As the `run` entry of a kernel gate (the bead or
  final gate of an enrolled spec) it takes the subject from the gate, reads the round's lens step from the gate round
  record (`lens`, §13.7), renders the BRIEF body (§13.11), and refuses another `--target`, a round past the allowance,
  a lens step not closed `passed`, and `--brief-file`. Every other `panel create` keeps `--brief-file` (§13.11).
- `panel submit`: closes the matching slot as `submit` does; it needs `--lease`, and §6's generation rule decides which
  generation it accepts.
- `panel refute` / `panel defer`: on an enrolled gate each needs `--actor <id>`, writes it as the record's `by` and
  in `mindspec_actors`, and refuses an actor that §13.10 refuses for the verify of that round and an actor recorded
  on any attempt of `<gate>.r<N+1>.fix`. The moved-subject guard
  (§13.2) holds its retally, except while the fix of a frozen round is open (§13.10). A passing retally closes the open verify or
  fix `superseded` and closes the gate `passed` with the review subject. After an open-gate `re-panel` judgement (§13.2),
  it does not close the gate: it authorises round N+1 of n slots as a passing tally does.
- `brief <step>` (new): prints the brief of an actor step of an enrolled spec (§13.11) and writes nothing. It refuses
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
receipt): with evidence and a generation §6 accepts, `submit` makes the close writes of its kind and writes the
receipt (no reopen, §6). Else, when the step was leased (generation 1 or more) and its
evidence resolves at a commit and fits its kind and current subject (a slot's verdict at its registered path; an
implement or fix commit past its claim base or its step subject (§6) on the branch the gate names, as §6 checks it; a lens or
verify record at `<spec dir>/kernel/<step id>.json`; a judgement in a registered panel directory whose new
subject is the judge's subject), the pass lists `submit <id> --take --actor <actor> --reason <text> (--artifact |
--commit) <evidence>` in `run` and creates no retry. That submit needs no token and runs every check of §6 and the
§13.10 actor refusals on `--actor`. It issues no generation: it appends a take event (`via: "receipt"`, `generation` =
the step's current generation, and `actor`) and a `mindspec_actors` entry (`via:
"receipt"`), makes the close writes of its kind (§6), and then writes the receipt (`repaired: true`, `taken: true`,
`actor` = `--actor`). Only when the step was never leased, no evidence
resolves and fits, or its `mindspec_closing` has a `conflict` (§6), does the next pass create `<id>.a<k+1>` with `mindspec_from`. `submit --take` refuses an open step,
a step that was never leased (so no record written outside a lease passes as a take), a step with a valid receipt and
evidence that does not resolve or fit, and names the condition. A retry keeps the keys
of its kind: subject, round, registration and decision, for a fix `mindspec_reproduced`, and for a verify
`mindspec_findings` and `mindspec_read` (a retry verifies the same findings; a changed verdict file retires a verify
instead, §13.10); the pass points
`mindspec_round.lens` or `mindspec_round.verify` at a lens or verify retry. The pass, `panel create` and `submit`
read the highest attempt not closed `superseded` of a slot or fix step and the attempt that `mindspec_round` names of
a lens or verify step; the §13.10 independence checks read every attempt.

## 10. Rounds and decisions

`panel.max_rounds` defaults to 5. `loop.halt.max_rounds_per_bead` is a warned alias for one release, read only when
`panel.max_rounds` is unset; the warning names the value. `panel_deadlock_rounds` and `max_consecutive_impl_failures`
are deleted. Rounds are counted per gate over every panel registration of the work item, abandoned included, from the
git ledger (not per directory). Redispatch (`panel recover` / `panel launch`) adds no round; the §13.10 cap bounds the
redispatch of an uncounted slot. At the limit: a decision
`continue:<n>` (raises the allowance; history kept) or `halt`. A REJECT gives a decision for that item only: `fix`
(counted) or `halt`. There is no `accept`: findings are fixed, refuted or deferred. `halt` closes open steps and
leaves the epic and work items open (§13.4). Spec and plan gates: count and warn.

## 11. Audit table

`impl approve` prints the table to stderr; the table never blocks approval. It shows policy and counts of each escape hatch (`--allow-doc-skew`,
`--override-adr`, `--allow-not-ready`, `--allow-net-deletion`), deferrals, takeovers, timed decision answers, raw
claims, unreceipted closes, carry-overs, the slot count of each re-panel, and six new rows, from durable records.
`--allow-not-ready` joins both friction registries. Each row is `Event | Count | Details`, and Count is the number of
records the row names. The new rows: `verify-reproduced`, `verify-refuted` and `verify-deferred` (one row per return
type; Count = returns of that type in the `mindspec_returns` of every verify attempt); `late-submissions` (receipts
with `superseded_generation`, §6); `take-receipts` (receipts with `taken: true`, §9); `lens-steps` (Count = lens
attempts of all gate rounds; one Details entry `<gate> r<N>: <k>` per gate round with k attempts).

Takeover history is stored in `mindspec_takes`: one event for each `lease --take` (`via: "lease"`) and for each receipt
under `submit --take` (`via: "receipt"`). The takeovers row counts every event of both kinds, whatever its generation.
The unreceipted-closes row counts every receipt with `repaired: true`, so a receipt under `submit --take` counts in
that row, in the takeovers row and in its own row. Actor history is stored in `mindspec_actors` (§5).
Deferral history is stored in `mindspec_deferrals`, with distinct panel, round, slot, finding and target records. The
deferrals row counts every record in it: each `panel defer` and each `deferred` verify return (§13.10).
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
   gate is the review subject (§6) of its gate round record (`mindspec_round`) when that round is registered, else, for a
   scoped round, of the record's `prior` (§13.7), else the record's authorised subject (for a successor with no round, its frozen
   predecessor's accepted subject). A commit that changes only a panel directory of any
   registration of the gate, or the kernel directory (§13.11), does not move the subject, also before the gate's first
   registration. A pass creates the actor step `<gate>.judge.a<k>` (`<gate>` is the base id `<w>.review` or
   `<epic>.final` for every attempt; kind `change-judge`, child of the gate's parent, subject = the newest subject
   commit), leased and submitted as §5 and §6 say, when the newest subject commit is not the accepted subject and no
   judge of the gate (any attempt, open or closed `passed`) has that commit as its subject, in each of these cases:
   (a) a tally or a verify retally (§13.10) of an open gate is due: every slot step of round N is closed `passed`, no
   `uncounted` decision of round N is open and round N is not frozen, or round N's current verify is closed `passed` and not applied (§13.10); (b) the gate is passed (highest attempt not closed
   `superseded`), the work item of a bead gate is not merged and `impl approve` has not closed the epic; (c) a
   `re-panel` round is authorised and not registered (§13.7). So a closed judgement ends its subject change: the
   scoped round it authorises, that round's tally and a verify retally on the judged subject create no judge, and only
   a later subject commit does. **Moved-subject guard:** while a judge of the gate is open, or the rule above makes one,
   the `tally`, `refused-round` and `verify-apply` rules and a `panel refute` or `panel defer` retally (except while the fix of a
   frozen round is open, §13.10) do not act on the gate; the outcome waits for the judgement, and the verify and the fix take the review subject (§6). One subject change has one live judge: a pass closes an open
   judge `superseded` when its subject is not the newest commit or is the accepted subject (a successful rebind; the
   receipt names the rebind), then applies the rule above, so any `.a<k+1>` judges the newest commit. The artifact, in a registered panel directory of the gate like a verdict, is a judgement
   with one of two values: `carry-over` with a reason, or `re-panel` with a slot count n and the changed paths. n is
   from 2 to the slot count of the gate's passing round, or for an open gate of the round its tally covers; after a
   1-slot round only `carry-over` fits. `submit` checks the shape, the old subject (the accepted subject), the new
   subject and, for a passed gate, the frozen gate's receipt. It computes the changed paths, `git diff --name-only`
   from the old to the new subject without the panel directories and the kernel directory, and refuses a `re-panel`
   whose paths do not include each of them, naming each missing path. It never judges the content. A judgement that
   does not fit is refused, step open. Outcomes:
   - Open gate, either value: the gate round record sets `judged` to the judge, the value and the new subject, which
     becomes the review subject (§6); `subject` stays the round subject. When the round has a current verify
     (`mindspec_round.verify`) that is not applied, the pass retires it (§13.10) and creates `<gate>.r<N>.verify.a<k+1>` on the review
     subject.
   - Open gate, `carry-over`: the waiting outcome applies to the review subject: a passing tally closes the gate
     `passed` with it; a refused tally creates the verify on it. An incomplete tally has no waiting outcome: the
     judgement only sets `judged`, and the later tally of round N applies it.
   - Open gate, `re-panel`: the gate takes the re-panel record. A refused tally creates the verify on the new subject.
     A passing tally (`tally`), a verify that leaves no reproduced finding (`verify-apply`), or a passing `panel refute`
     or `panel defer` retally (§7) authorises round N+1 of n slots on the new subject on the same gate, with the paths
     as its scope, and does not close the gate: that rule writes the round N+1 record (§13.7 `prior`), not `round-open`. A verify that leaves a reproduced finding creates the fix, and the
     round after the fix is a full round, unless a passing `panel refute` or `panel defer` retally closes that fix
     `superseded` and authorises round N+1 of n slots (§13.10).
   - Passed gate, `carry-over`: the gate stays `passed`; its receipt adds the old subject, the new subject and the
     judge's receipt, and verbs accept the new subject.
   - Passed gate, `re-panel`: the pass creates the successor `<gate>.a<k+1>` (same parent, `mindspec_from` = the
     frozen gate, the re-panel record) and authorises a round of n slots on the new subject; its `panel create` entry
     in `run` (§13.7) carries the changed paths as the round's scope.
   Verbs select the highest attempt not closed `superseded`, rounds keep counting, and §2 ids apply to it. Judge and
   gate attempts count apart. An `impl approve` killed after its finalize export commit, before the epic closes, leaves
   a final judge that `carry-over` clears. Applied readings, not in the ruling: the accepted subject and its
   precedence, rebind's pass, the panel-directory exemption, the end state, judge parent and id, supersede of a judge,
   separate attempt counters, slot ceiling, scope carrier. From the 2026-11-21 amendment: the accepted subject of an
   open gate; a scoped round on the same open gate (no successor); the round number of an open-gate re-panel round
   (§13.7) and the `prior` record that its authorising rule, and `round-open` after a closed fix, write, with
   `verify` only for a verify closed `passed`, `read` and `asks` in bd metadata and no git object, and a dry pass that writes nothing (§13.7); the slots a scoped round takes, with no or two
   adversary slots, and their contiguous ids (§13.11); the review subject, `judged`, and the verify attempt after a
   judgement, so a verify on the old subject is redone (a refutation holds for the subject it read; B5(c)); the key
   `mindspec_round.verify`, the retirement triggers of a verify, and its refutations and deferrals kept as audit
   history only (§13.10); a fresh lens attempt on each authorisation or subject move, and the retired passed attempt (§13.7);
   the judge-termination rule (no judge on a commit that a
   closed judge ruled on), the moved-subject guard and when a tally or verify retally is due (case a); the kernel directory, its tracked location outside `reviews/`, its
   exemption before the first registration, the exclusion of all of `<specs>/*/kernel/**` (a file there is not
   reviewed) and from docsync's documentation files, the finalize export of an uncommitted record, and the lens and verify close that copy the record into bd metadata, before the receipt on every receipt
   path (§6); the closing pin `mindspec_closing` on a verify only (a lens has none, as its one write is replaced whole,
   so a take after a lens crash needs no retirement), run after the token and shape checks and only on a step with no
   valid receipt, the refusal and retirement on a `conflict`, the matching rerun that repeats its writes, the
   late-submission rule that reads it, and no discharge by a verify with no valid receipt (§6, §13.10); `verify_step`
   in the ledger key of a refutation or deferral (§13.10); the metadata bound: a uniquely named `@<file>` (mode 0600,
   removed after the call, stale files removed by the next pass) on every metadata write, `Create` included, full
   stored texts, and the 16 MiB `metadata-too-large` kernel limit, checked before the pin, and its recovery (§13.10); `prior.asks` and `prior.read` from `prior.verify` when set,
   and `prior.read` as audit history (§13.7); the fix check that a record commit cannot meet (§6); the `lens` key
   (§13.7); the subject that §6's late-submission rule compares; `--actor` on
   every new lease generation, on `next` and on `submit --take`, and `mindspec_actors`; declared actor labels as the B4
   and B7 mechanism, and the exclusion of the reviewed commit's author steps and of the next fix's actors from a verify (§13.10); the generation of a receipt take
   event and the audit row counts (§9, §11); the `veto` finding, its returns with no `deferred`, and the gate pass after
   a verify (§13.10); a veto finding for each off-roster REJECT, `hard_block` or INVALID file; no verify on an incomplete tally, and the next slot
   attempt for an uncounted roster verdict, its cap of 3 uncounted attempts and the one `uncounted` decision that lists
   every capped slot with its highest attempt, and the lease hold it shares with §13.8 (§13.10); the applied verify, a decision counted as its outcome, and the frozen round after it,
   which case (a) does not judge and whose retally binds the panel to round N's round subject, closes on its review
   subject, is covered by its verify record and compares the verdict files on disk with the set that the verify step
   pinned at its creation (§13.10); the verify
   receipt time that ends earlier verb refutations and deferrals, and a same-key dedupe, with each verb's key, against kept
   records only (§13.10); the current verify of a `prior` with no verify; a new limit decision for a round that a retally
   authorises past the allowance, its attempt id and its `continue:<n>` that creates that round's lens step (§13.8); one new verify attempt
   when a withdrawal and a judgement retire it in one pass (§13.7); a reproduced off-roster veto as a code
   finding (§13.8); a veto verdict that gives no `ccr` finding; the verify record fields for a refutation or deferral,
   the `verify_step` field and the deferral check before the receipt; `--actor` and the independence check on `panel
   refute` and `panel defer` of an enrolled gate, which refuses every actor that the verify of that round refuses, as a
   verb refutation discharges a finding as a verify return does (§7, §13.10); the new verb
   `brief <step>` (§7); the `--brief-file` scope (§7); `<actor>` and `<text>` in `run`
   entries (§4); the `submit --take` refusals and the keys a retry keeps (§6, §9); the
   never-leased step refused by `submit --take` and retried, a narrowing of ruling E to leased steps (§9); the audit row names (§11);
   `panel tally` advisory on an enrolled gate; the adversary slot found by its roster lens; the lens step of a round
   with no lens slot; unknown fields refused in lens and verify records; the slot `focus` carrier and its precedence
   (§13.11); files in scope, the prior-round asks rendered from `prior.asks` alone, and the
   `<epic>.final` brief sources (§13.11).
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
   that leaves no reproduced finding or a passing `panel refute` or `panel defer` retally (§13.2: n slots, on the successor of a passed gate or on the open gate); for
   `<epic>.final`, the last work item done (B4; round 1, the spec branch head). On an open gate a scoped round is
   round N+1, where N is the gate's last registered round. The rule that authorises a round writes the gate round
   record (`mindspec_round`: round, authorised subject, slot count, scope, `lens`, `verify`, `judged`) and creates the
   lens step `<gate>.r<N>.lens` (§13.11), for a scoped round too. Each rule that authorises round N+1 on an open gate
   (`round-open` after a closed fix, `tally`, `verify-apply` or a passing `panel refute` or `panel defer` retally,
   §13.2) copies the round N record, without its own `prior`, into `prior`; `prior.verify` names round N's current verify
   only when that verify is closed `passed`, so a retally that closes it `superseded` writes no `verify`. The rule also
   writes `prior.asks`, the rendered asks (§13.11), and `prior.read`. When `prior.verify` is set, `prior.asks` takes
   each round-N finding (slot, round, finding, `finding_sha256`) and its text only from that verify's
   `mindspec_findings`, joined by that identity with its `mindspec_returns` (return, ground, reason, evidence or target)
   and with each `panel refute` or `panel defer` record with an empty `verify_step` that the §13.10 filter keeps, and
   `prior.read` is a copy of its `mindspec_read`; the rule reads no verdict file. Else `prior.asks` is each round-N
   finding that the rule enumerated from the files it read on disk (for a tally or retally, the files it tallied),
   joined with those verb records, and `prior.read` is each round-N verdict file and the `panel.json` of round N's
   panel directory as repo path -> sha256 of its bytes. `prior.read` is audit history only: no rule compares it.
   Stored texts are full, under the §13.10 bound. It writes no git object. Review
   directories are ignored and local, so `prior` holds what authorised round N+1 in bd metadata, and no rule reads it
   back from a commit. A later rewrite of the round N+1 record and `panel create` keep `prior`. The round N+1 brief
   renders from `prior` alone, and the §13.10 filter reads round N's current verify from `prior`. A pass without
   `--apply` computes the digests and the asks and writes nothing (§4). At the round N+1 authorisation the pass closes each open slot
   attempt of round N `superseded`. Each later
   authorisation of round N (a new slot count or subject included) and each move of its authorised subject before
   `panel create` creates `<gate>.r<N>.lens.a<k+1>` on the current authorised subject (`mindspec_from` = the prior
   attempt): the pass closes an open prior attempt `superseded`; a prior attempt closed `passed` keeps its receipt and
   is retired. The key `mindspec_round.lens` names the round's current lens step; `panel create` reads it and no other
   attempt. When that lens step is closed `passed`, the pass lists `panel create` for that gate in
   `run`; its pass creates the slots. While a round
   on the current subject has an open slot, `panel create` refuses and names it (redispatch: `panel recover` or
   `launch`). A subject move before `panel create` updates the authorised subject and gives a fresh lens attempt (above),
   except after a `re-panel` judgement: a later non-panel commit before `panel create` opens a new judge on the newest subject
   (§13.2 case c), which rules on the whole diff from the accepted subject. Its outcome replaces the scoped round: the
   pass closes the unregistered successor `superseded` (on an open gate, it withdraws the authorised round: `prior`
   becomes the gate round record again, so the verify it names is no longer applied, and §13.2 "Open gate, either
   value" retires it and creates
   one new attempt on the new review subject, so one pass creates no transient attempt), closes its open lens attempt
   `superseded` or retires a passed one, then applies §13.2 to the frozen or open gate; a new authorisation gets a fresh
   lens attempt. A panel-only commit does not move the
   subject. The re-panel record states what the judge authorised; the gate round record alone states the slots that
   run. A round past the allowance gives the limit decision, not a lens step or `run` entry.
8. **REJECT (OQ13).** A complete tally with a REJECT verdict is a refused round, so it creates the verify (§13.10).
   Below the limit, a verify that returns `reproduced` for the `veto` finding of a REJECT verdict (§13.10) creates the
   fix `<gate>.r<N+1>.fix` and the decision `<gate>.decide.reject.r<N>` (`fix` or `halt`); a refuted REJECT veto gives
   no decision, and another reproduced veto (`hard_block`, or an off-roster INVALID file) creates the fix with no
   decision. A reproduced veto is a code finding: its fix changes the code the verdict names, and a commit that only
   moves a verdict file does not meet the §6 fix check. While the
   decision is unanswered, a verb that would claim or close another child of that gate, open a round on it, or close or
   merge what it decides refuses and names it; `next-action` lists the decision, not the fix, until `--answer fix`
   releases the fix. A limit decision holds only a verb that opens a round or claims or closes a fix; a passing `panel
   refute` or `panel defer` retally acts as §7 says and closes the limit decision `superseded`; a round N+1 that it
   authorises past the allowance gives a new limit decision (§13.7), `.a<k+1>` after an earlier limit decision of the
   same round (§2); as no fix exists, its `continue:<n>` raises the allowance and creates that round's lens step. The answer adds no
   round. At the limit, a verify that leaves a reproduced finding creates only the limit decision, and `continue:<n>`
   raises the allowance and creates the fix.
9. **Pass order (OQ12).** A verb runs its pass first (leaving out the step it names), then its refusal checks, then
   its own write, then the rest of the pass. On a refusal the first part stands and the next pass covers the step.
10. **Verify.** A complete, refused tally, after any judgement (§13.2), creates the actor step `<gate>.r<N>.verify`
    (kind `verify`, child of the gate, subject = the review subject, §6), leased and submitted as §5 and §6 say. It
    runs at every gate B9 covers, `<epic>.final` included, below and at the limit. A verify is not a round and adds
    none. **Incomplete tally.** When every slot step of round N is closed `passed` and the tally does not count the
    verdict of a roster slot (missing, unsubmitted, malformed, draft, digest mismatch, VOID, or INVALID), the pass
    creates that slot's next attempt `<gate>.r<N>.R<s>.a<k+1>` (`mindspec_from` = the passed attempt, which keeps its
    receipt and is retired) and no verify; the run entry is the §10 redispatch. An uncounted verdict is not a review, so
    it is not a finding. The round tallies again when each new attempt closes `passed`. **Cap.** A slot's count is the number of
    its round-N attempts closed `passed` and uncounted that are numbered above the attempt recorded for it by the highest
    `uncounted` decision of round N that lists the slot and was answered `redispatch` (all its attempts when no such
    decision exists). A
    retired attempt (closed `passed`, with a later attempt) is uncounted, as only the incomplete-tally rule retires it;
    the highest attempt is uncounted when the tally does not count its verdict. When the incomplete-tally rule fires and
    one slot or more has a count of 3, the pass creates no slot attempt of round N; it creates one decision
    `<gate>.decide.uncounted.r<N>` (later ones `.a<k+1>`; options `redispatch`, `halt`), written by `decisionWrite`.
    The `slots` field of its `mindspec_decision` record lists each slot with a count of 3 and its highest attempt, in
    roster order (`[{"slot": "R2", "attempt": 3}]`). While it is open, the pass creates no slot attempt of round N, the
    tally of round N does not act (the gate has an open child), §13.2 case (a) is not due, and `lease` refuses every
    step of the gate (§5, §13.8; `lease.go` holds a gate while a decision of it other than `limit` is open). `redispatch` creates the next attempt,
    on the round subject (B10), of each listed slot whose verdict is still uncounted; `halt` is §13.4. **Refused
    round.** The `refused-round` rule (`refusedRound`, `reject.go`) fires when every slot step of round N is closed
    `passed`, the tally is complete (`Complete()` and no roster verdict INVALID) and it does not pass; it skips a round
    mismatch, `PassRule`'s intact check and an unreadable registration. **Findings.** Round N's findings are one `veto` finding
    for each verdict file of round N that is REJECT, has `hard_block`, or is an off-roster file (one that `verdictFileRE`
    matches with no roster slot; slot = its name stem) that is REJECT, `hard_block` or INVALID, as `PassRule`'s
    `unexpectedVeto` reads it (another off-roster file blocks no tally and gives no finding); and each `ccr[<i>]` of the `concrete_changes_required` of every
    other roster verdict. A veto verdict gives no `ccr` finding (`panel refute` and `panel defer` refuse a REJECT or
    `hard_block` verdict); the fix brief carries its `ccr` texts. A schema-1 REQUEST_CHANGES verdict with no
    `concrete_changes_required` gives the finding `slot`, as `panel refute` names it. `PassRule` also refuses a
    round mismatch and its intact check (`Result.intactError`: fewer than 2 expected reviewers, `panel.json`
    integrity); for those, and an
    unreadable registration, the gate has no action until the operator repairs the panel directory. Every other refusal of a
    complete tally is a REJECT, a `hard_block`, an off-roster veto or an unresolved REQUEST_CHANGES, and each gives a
    finding. So a verify
    is created only for a refused, complete tally, and has one finding or more: `refusedRound` checks both, and it
    writes nothing and reports an internal error when the enumerator returns no finding. When it creates a verify
    attempt, the pass pins in the step metadata `mindspec_findings`, the enumerated findings with their text, and
    `mindspec_read`, each round-N verdict file it read from disk as repo path -> sha256 of its bytes; the verify brief
    renders from them. The pin is made once, at the verify's creation, so a round-N verdict file edited before the
    verify closes fails the frozen-round retally below until the fix closes. **Metadata bound.** Every
    `kernel.Store` call that sends metadata to bd (`Create`, `WriteMetadata`, `SetStatusMetadataIf`) gives bd the
    merged metadata as `--metadata @<file>`, never as one argv string, which Linux refuses over 128 KiB; bd reads
    `@<file>` on create and update (`bd create --help`, `bd update --help`, bd 1.3.4). The file has a unique name per call (it
    names the process id), mode 0600 and a kernel temporary directory outside the repository, and is removed after
    the bd call returns; each pass removes the files there whose process no longer runs. The kernel stores each text in full
    (finding texts, `reason`, `evidence`, `target`, `ac`, lens texts); no stored field is cut. Before any bd call the
    store refuses merged metadata over 16 MiB (a kernel limit, not a bd limit) with the error `metadata-too-large`, which names the gate, the step, the
    largest key and the size, and writes nothing. A verify submit builds and size-checks each close write, `panel.json`
    included, before it writes `mindspec_closing` (§6), so an oversized record leaves the step open with no pin. The pass reports the error for that gate and acts on no later rule of it.
    Recovery: when the oversized text is in the verify record, the actor submits a shorter record; when it is in a
    round-N verdict that no verify has pinned, the operator shortens that
    verdict file; its digest then fails, so the tally does not count it and the pass creates that slot's next attempt
    (incomplete tally, above); else the operator halts the spec (§13.4). Sizing: 12 slots x 100 findings of 4 KiB, each
    with a return and two verb records, is about 9 MB (arithmetic, not a measurement). `finding_sha256` is as `panel.Findings` computes it, or for `veto` the sha256 of the verdict file's bytes.
    APPROVE verdicts and the `findings` list outside `concrete_changes_required` give no finding. **Record.** The
    artifact `<spec dir>/kernel/<step id>.json` is a verify record: `{"step", "subject", "returns": [{"slot",
    "round", "finding", "finding_sha256", "return", "ground", "ac", "reason", "evidence", "target"}]}`. `submit` checks
    shape and existence only, never content: it refuses an unknown field (as for a lens record, §13.11); the returns cover each finding exactly once, by slot, round, finding and
    `finding_sha256`, and the list is not empty; `return` is `reproduced`, `refuted` or `deferred` (`veto`: `reproduced`
    or `refuted` only). A `refuted` return carries `ground` (`disproven`, `not-load-bearing` or
    `out_of_acceptance_scope`, the last with `ac`), a non-empty `reason` and `evidence` that resolves to a commit, a
    repo path or an https URL, as `panel refute` checks them; its `by` is the receipt's `actor`. A `deferred` return
    carries a `target` bead id and a non-empty `reason`; `submit` runs the `bead.CheckDeferral` checks on it before the
    receipt. It refuses with the step open and names each missing, repeated or unknown finding, each unlisted return,
    each failed field and each failed deferral check. On the close it records each `ccr` or `slot` return that is
    `refuted` as `panel refute` records a refutation and each `deferred` return as `panel defer` records a deferral
    (`mindspec_deferrals` included), each with `verify_step` (a new field of `panel.Refutation`, `panel.Deferral` and the `mindspec_deferrals` entry)
    naming the verify step, and writes every return to the step metadata `mindspec_returns`; its receipt records `at`, the close time (`mindspec_closing.at`, §6),
    written RFC3339Nano UTC from `kernel.Now`, the clock from which the enrolled `panel refute` and `panel defer` write
    `created_at`; a `veto` return lives in `mindspec_returns` and the verify-returns audit row only. No later rule
    reads the record file. A
    `veto` takes no `deferred`: `panel defer` refuses a REJECT or `hard_block` verdict, and a deferred veto would pass a
    REJECT that no one reproduced or refuted.
    **Current verify.** The key `verify` of the gate round record of round N (or of its `prior`, §13.7) names round
    N's current verify attempt. Before it is applied, the pass retires it when a judgement changes the review subject
    (§13.2), when its `mindspec_returns` no longer cover round N's findings (a verdict file changed after it closed), or
    when its `mindspec_closing` has a `conflict` (§6; the new attempt keeps `mindspec_findings` and `mindspec_read`), and points
    the key at the new attempt `<gate>.r<N>.verify.a<k+1>` on the review subject (`mindspec_from` = the retired
    attempt); on an incomplete tally the pass makes no new attempt and clears the key, and `refused-round` makes the
    attempt and writes the key when the tally is complete and refused again. A retired attempt that is open closes `superseded`; a closed one keeps its receipt. A
    verify is applied when it is current, closed `passed`, and `verify-apply` acted on its receipt (it closed the gate,
    created the fix or a decision, or authorised a round that is still authorised or registered). **Frozen round.**
    Once round N's verify is applied, round N is frozen: the `refused-round` rule, the incomplete-tally rule, the
    changed-finding trigger and §13.2 judge case (a) do not act on round N, a later change to its verdict files is audit
    history only, and the next round reviews the subject. Only a re-judge that withdraws the round the verify authorised
    (§13.7, ruling C) retires an applied verify; that verify created no fix, as a verify that leaves a reproduced
    finding authorises no round. The verify attempt after that withdrawal enumerates round N's verdict files as
    they are then. An open fix of
    round N+1 ends only by its own close or by a passing `panel refute` or `panel defer` retally (§7). While that fix is
    open, the moved-subject guard does not hold that retally. It reads round N's verdict files in the panel directory on
    disk, as every tally does, and binds the tally to round N's round subject (the panel's subject must equal it), not
    to the head commit. It passes only when `PassRule` passes and round N's current verify covers it: its `mindspec_returns` cover every
    round-N finding that the enumerator lists (slot, round, finding, `finding_sha256`), and the round-N verdict files on
    disk now are the set in that verify step's `mindspec_read`, each with the same sha256 (a changed, added or removed
    file fails it, committed or not, and the refusal names each path, escaped with `termsafe`). When it passes, the fix closes
    `superseded` and the gate closes `passed` on round N's review subject, and §13.2 case (b) then judges the later
    commits; after an open-gate `re-panel` judgement, which came before the verify, it authorises round N+1 instead
    and does not close the gate (§7, §13.7). The refutations and deferrals of a verify with no valid receipt or of a retired verify (their `verify_step` is set and is not the
    current verify of their round: the `verify` key of round M's record when `mindspec_round` or its `prior` is round
    M (no current verify when that key is empty), else the highest attempt of `<gate>.r<M>.verify`, which no rule retires once round M+1 is authorised), and, once
    a verify attempt of round M is closed `passed`, each refutation or deferral of a round-M finding with an empty
    `verify_step` whose `created_at` is not after the receipt `at` of the highest such attempt (one controller host's
    clock, §8; compared as parsed times, so an equal time is not after), discharge no finding and stay as audit history only: the kernel removes them from a deep copy (`Result.Panel`, a pointer, and
    its `Refutations` and `Deferrals` slices; the tallied `Panel` is never edited, as `panel refute` writes it back), so
    `PassRule` does not change, before it calls `PassRule` in
    `tally` and `refusedRound` (`reject.go`), `gatePassEvidence` (`reducer.go`), `CheckPassedGate`, and the enrolled
    `panel refute` and `panel defer` retally (`cmd/mindspec/panel_kernel.go`). The ledger key of a refutation or deferral (`recordKey`,
    `internal/panel/disposition_store.go`: slot, round, finding, `finding_sha256`, for a bound deferral also
    `source_verdict` and `source_file_sha256`) adds `verify_step` when it is non-empty, and `created_at` when the
    record has the field `verify_step` empty (an enrolled verb writes it empty; a pre-kernel record omits it and keeps
    today's key). So on an enrolled gate no two verify attempts or verb calls share a key, and a discarded record never blocks
    an append. On
    an enrolled gate the same-key dedupe (slot, round, finding, `finding_sha256`; for `panel defer` also
    `SourceVerdict` and `SourceFileSHA256`) of `panel refute` and `panel defer`, and of the `mindspec_deferrals`
    history that `panel defer` appends, runs before the append and compares only the records that this filter keeps.
    The verify close does not dedupe; its matching rerun appends identical records (§6).
    **Pass after a verify.** A gate closes `passed` (receipt `retally: "verify"`) when its current verify closed
    `passed` with a valid receipt, its `mindspec_returns` cover every finding of round N, the `veto` findings included, and no
    return is `reproduced`. So a refuted REJECT passes the gate (ruling B3, B5). `passed()`, `gatePassEvidence` and `CheckPassedGate` accept this receipt;
    `panel.PassRule` and the `panel refute` and `panel defer` refusals of a REJECT or `hard_block` verdict stay as they
    are, and on an enrolled gate `panel tally` is advisory: its output and exit code do not change, and `next-action`
    decides the gate. No fix is created (under a `re-panel` judgement: §13.2).
    Else B9 and §13.8 apply, and the fix's metadata `mindspec_reproduced` lists the reproduced findings (slot, round,
    finding, `finding_sha256`) as its scope. A passing `panel refute` or `panel defer` retally (§7) closes an open verify
    `superseded`. The enumerator lists every finding, those that a `panel refute` or `panel defer` already resolved
    included; the verifier returns them again. **Independence** is a shape check on the declared actors in `mindspec_actors` (§5, §9): `lease` of a
    verify, and `submit --take` of a verify, refuse an actor recorded on any slot of round N, an actor recorded on any
    attempt of `<gate>.r<N+1>.fix`, and an actor recorded on a step that authored round N's subject (round 1: every `<w>.impl.a<k>`; round N>1 after a fix: every attempt of
    `<gate>.r<N>.fix`; a scoped round: none, as no kernel step authored its subject; for `<epic>.final` round 1, the fix
    and implement steps of the spec's work items). The `--actor` of `submit --take` is the step's actor for this check;
    the run entry carries `<actor>` for the orchestrator to fill with a label that passes it. A review-slot lease does
    not refuse the subject's author: the rulings name no such check, and the managed header is the slot's floor.
    `lease` and `submit --take` of the fix `<gate>.r<N+1>.fix` refuse an actor recorded on any attempt of
    `<gate>.r<N>.verify`, and the enrolled `panel refute` and `panel defer` refuse every actor that a verify of
    round N refuses (§7). Each refusal
    names the step that holds the actor. The binary compares labels; it authenticates neither labels nor evidence
    authorship (§8, threat model). An approval count never resolves a finding: each finding is fixed, refuted or
    deferred.
11. **Briefs and lenses.** The binary renders every brief of an enrolled spec from durable records; no actor writes a
    brief. The `brief` argv of each action: implement, `next <w> --emit-only` (work item, spec, plan); review slot,
    `panel prompt` (the managed header, then one generated body for all slots of the round: a summary from the work
    item; the files in scope; the prior round's asks: round N's findings, rendered from `prior.asks` alone (§13.7), each with its text and its
    verify return, evidence or target, or the verb records that the §13.10 filter keeps, so a later edit of the verdict
    files, `git gc` or another clone does not change them; and the line "Do not raise a refuted or deferred finding again without new evidence"; then the slot's
    Focus); lens, verify, fix and change judge, `brief <step>`. **Files in scope:** for a scoped round, the round paths;
    for a full round, `git diff --name-only` from the claim base to the round subject (G1), and for round N>1 also the
    paths changed since the prior round's subject, marked as such. The claim base is the passed implement step's claim
    base, or for `<epic>.final` the final base. **`<epic>.final` sources:** the summary is from the spec, the
    work-item context is the epic's work items, and the final base is the merge base of the spec branch with the branch
    that `impl approve` merges into. A lens brief gives the spec, plan, work item (for `<epic>.final`, the work items)
    and the diff of the files in scope at the authorised subject; a verify brief gives round N's findings (slot, finding, text) from `mindspec_findings` and the subject; a fix
    brief gives the findings in `mindspec_reproduced` (with the `ccr` texts of a `veto` verdict), the subject and the
    work-item context; a change-judge brief
    gives the accepted subject, the new subject, the diff between them and the slot ceiling (§13.2). **Lens step.**
    When a round is authorised, the pass creates `<gate>.r<N>.lens` (or a later attempt, §13.7; kind `lens`, child of
    the gate, subject = the authorised subject) before the `panel create` entry, for a scoped round too. The artifact
    `<spec dir>/kernel/<step id>.json` is a lens record: `{"step", "lenses": [{"slot": "R<s>", "lens":
    "<text>"}]}`. **Scoped round roster.** A scoped round of n slots selects from the gate's configured slots, in roster
    order, each adversary slot (configured lens `adversarial`) up to n, then the first other slots up to n. The round
    roster lists the selected slots in configured order and renumbers them `R1`..`Rn`; each keeps its configured
    family, model and lens. So the ledger, `panel.json`, the slot steps and every check that requires `R<i+1>` stay
    unchanged; on the default roster (adversary fourth) a 3-slot round is R1, R2 and R3, where R3 is configured slot 4
    with lens `adversarial`. The adversary slot of a round is the roster entry whose lens is `adversarial`. A roster
    with no adversary slot gives every selected slot a lens, and the managed header is the floor. A round whose slots
    are all adversary slots still has a lens step; its record has an empty `lenses` list. `submit` checks shape only:
    it refuses an unknown field, and needs exactly one non-empty lens for each slot of the round except an adversary
    slot, and none for it; it refuses and names each missing, extra or empty entry. The lens close writes the
    lenses to the gate round record before the receipt (§6): its one write key is `mindspec_round.lenses`, which each
    run replaces whole, so a lens retry writes its own lenses and a lens has no closing pin (§6). `panel create` refuses while the step that
    `mindspec_round.lens` names is not closed `passed` and names it; it writes each lens from `lenses` to the slot's
    `focus` field of `panel.json` (`RosterSlot`, written
    only by the kernel's `panel create`). `panel prompt` prints a slot's Focus from its `focus` when set, else from the
    configured lens text, as today. Coverage floor: each slot keeps the managed header (the spec's acceptance criteria
    and the acceptance question); a lens adds focus and replaces neither. The adversary slot has no `focus` and runs
    its configured lens. **Kernel directory.** `<spec dir>/kernel/` holds lens and verify records. It is tracked, outside the
    ignored `reviews/`, and the actor commits the record on the branch the gate names (`bead/<w>`, or the spec branch
    for `<epic>.final`), as `submit` resolves it at a commit (§6). `panel.Subject` appends the exclusion
    `<specs>/*/kernel/**` after the four it has, and the effective subject, the §6 fix check and the change-judge paths leave it out also when
    the gate has no registration. So a commit that changes only kernel records never moves the subject and never meets
    the §6 fix check. Docsync does not count a kernel record as a documentation file. Committed records merge to the
    main branch with the spec as audit history; the finalize export commits a record left uncommitted (for example of a
    retired attempt) with its other artifacts, and no rule reads it. Every `panel create` that is not the `run` entry of a
    kernel gate (a pre-kernel spec, the spec and plan gates, `--gate adhoc` on any spec) keeps the BRIEF stub and
    `--brief-file`.
