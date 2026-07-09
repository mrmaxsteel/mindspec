// ms-panel.js — the /ms-panel Claude Code dynamic workflow.
//
// This script COORDINATES AGENTS. It never touches the filesystem or a
// shell itself (the documented workflow limit: "No direct filesystem or
// shell access from the workflow itself. Agents read, write, and run
// commands. The script coordinates the agents."). Every CLI invocation and
// every file write below is performed by an agent() step, never by this
// script's own code.
//
// Input (the documented `args` global — spec 111 R1):
//   {
//     slug,               // required, path-element-safe
//     spec,               // required, path-element-safe
//     target,             // required, branch-name-safe grammar
//     bead_id,             // optional, path-element-safe
//     round,              // required, positive integer
//     sha,                // optional, ADVISORY ONLY — never used to set the
//                         // recorded SHA; `panel create` self-resolves it
//                         // from `target` (110 R1)
//     lenses,             // array of per-slot lens strings the invoking
//                         // skill composed (judgment, R6/R7 — out of scope
//                         // here)
//     mix,                // required, non-empty [{family, count}] resolved
//                         // by the invoking skill from config `panel:`
//                         // (spec 109) — this workflow NEVER derives its
//                         // reviewer mix from anywhere else
//     claude_sub_on_quota, // optional boolean, resolved by the invoking
//                         // skill from config `panel.substitution.
//                         // claude_sub_on_quota` (spec 109). This script has
//                         // no filesystem/shell access, so it cannot read
//                         // .mindspec/config.yaml itself, and reading config
//                         // is intentionally outside ALLOWED_CLI (below) —
//                         // the invoking skill resolves the flag and passes
//                         // it in. Absent/not-exactly-true is treated as
//                         // false (fail-closed — R4).
//   }

// ---------------------------------------------------------------------------
// ALLOWED_CLI — the exact, exhaustive set of shell commands any /ms-panel
// agent step may exec. Adding a command here (or building one dynamically)
// is a gate-integrity change; TestMsPanelWorkflow_AllowedCLIExactSet enforces
// this set is exactly these four. The lifecycle merge-terminal verb is
// intentionally absent from this set and from this file entirely — this
// workflow is an adapter, never a lifecycle mutator (ADR-0037/0040).
const ALLOWED_CLI = [
  "mindspec panel create",
  "codex exec --sandbox read-only --skip-git-repo-check",
  "mindspec panel verify",
  "mindspec panel tally",
];
const [CMD_PANEL_CREATE, CMD_CODEX_EXEC, CMD_PANEL_VERIFY, CMD_PANEL_TALLY] =
  ALLOWED_CLI;

// buildCommand(verb, ...values) — the single command-construction
// chokepoint. `verb` must be one of the four identifiers destructured above
// (never a retyped literal); `values` carries only user-derived VALUES
// (slug/spec/target/bead_id/round) — never a flag. Each per-verb template
// below supplies its own fixed option flags; a value that looks like a flag
// (leading `-`) is rejected as an injection attempt before any template runs.
function buildCommand(verb, ...values) {
  if (!ALLOWED_CLI.includes(verb)) {
    throw new Error("buildCommand: verb is not one of the ALLOWED_CLI entries");
  }
  for (const value of values) {
    if (value === undefined || value === null) {
      continue;
    }
    if (typeof value === "string" && value.startsWith("-")) {
      throw new Error(
        `buildCommand: rejected a leading-dash value (argument-injection guard): ${JSON.stringify(value)}`,
      );
    }
  }

  switch (verb) {
    case CMD_PANEL_CREATE: {
      const [slug, specId, target, beadId, round] = values;
      let cmd = `${verb} ${slug} --spec ${specId} --target ${target}`;
      if (beadId !== undefined && beadId !== null) {
        cmd += ` --bead ${beadId}`;
      }
      cmd += ` --round ${round}`;
      return cmd;
    }
    case CMD_CODEX_EXEC: {
      // The codex entry's flags are already fixed in the ALLOWED_CLI entry
      // itself (--sandbox read-only --skip-git-repo-check) — no value slots.
      // Read-only is correct because codex itself never writes a file (see
      // runCodexSlot below); the wrapper agent's own Write tool does.
      return verb;
    }
    case CMD_PANEL_VERIFY:
    case CMD_PANEL_TALLY: {
      const [slug] = values;
      return `${verb} ${slug}`;
    }
    default:
      throw new Error("buildCommand: unreachable");
  }
}

// ---------------------------------------------------------------------------
// Input hardening — runs at workflow entry, before any command or write path
// is built. Any failure aborts the workflow (a thrown error) before Step 2
// runs: no agent step, CLI call, or file path is ever constructed from an
// unvalidated value.

const CONTROL_BYTE_RE = /[\x00-\x1f\x7f]/;

// The clean-single-path-element contract 110's CLI validators apply: reject
// empty, ".", "..", "/", "\", and control bytes.
function validatePathElement(label, value) {
  if (typeof value !== "string" || value.length === 0) {
    throw new Error(`invalid ${label}: must be a non-empty string`);
  }
  if (value === "." || value === "..") {
    throw new Error(`invalid ${label}: must not be "." or ".."`);
  }
  if (value.includes("/") || value.includes("\\")) {
    throw new Error(`invalid ${label}: must not contain a path separator`);
  }
  if (CONTROL_BYTE_RE.test(value)) {
    throw new Error(`invalid ${label}: contains a control byte`);
  }
}

const SHELL_METACHAR_RE = /[`;|&\n]|\$\(/;

function validateShellSafe(label, value) {
  if (SHELL_METACHAR_RE.test(value)) {
    throw new Error(`invalid ${label}: contains a shell metacharacter`);
  }
}

// bead_id: the same path-element contract as slug/spec, PLUS the
// shell-metacharacter guard (it flows into a built command line). Optional —
// absent/null is fine.
function validateBeadId(beadId) {
  if (beadId === undefined || beadId === null) {
    return;
  }
  validatePathElement("bead_id", beadId);
  validateShellSafe("bead_id", beadId);
}

// target: a branch-name-safe grammar (reject empty, control bytes, a leading
// "-", any construct `git check-ref-format` disallows, a trailing "/" or
// ".lock", or whitespace) PLUS the shell-metacharacter guard. Always passed
// to buildCommand as its own argv-safe token — appended as a single element,
// never concatenated into a larger string.
const GIT_REF_DISALLOWED_RE = /\.\.|[~^:?*\[]/;

function validateTarget(target) {
  if (typeof target !== "string" || target.length === 0) {
    throw new Error("invalid target: must be a non-empty string");
  }
  if (CONTROL_BYTE_RE.test(target)) {
    throw new Error("invalid target: contains a control byte");
  }
  if (target.startsWith("-")) {
    throw new Error("invalid target: must not start with a leading -");
  }
  if (/\s/.test(target)) {
    throw new Error("invalid target: must not contain whitespace");
  }
  if (GIT_REF_DISALLOWED_RE.test(target)) {
    throw new Error("invalid target: contains a construct git check-ref-format disallows");
  }
  if (target.endsWith("/") || target.endsWith(".lock")) {
    throw new Error("invalid target: must not end with a trailing / or .lock");
  }
  validateShellSafe("target", target);
}

function validateRound(round) {
  if (!Number.isInteger(round) || round < 1) {
    throw new Error("invalid round: must be a positive integer");
  }
}

function validateMix(mix) {
  if (!Array.isArray(mix) || mix.length === 0) {
    throw new Error("invalid mix: must be a non-empty array");
  }
  for (const entry of mix) {
    if (!entry || typeof entry !== "object") {
      throw new Error("invalid mix entry: must be an object with family and count");
    }
    // The workflow's own slot ids (R1, R2, ...) are generated from a fixed
    // internal enumeration in flattenMix below — never interpolated from
    // args — but the reviewer family name IS user-derived input and is
    // hardened the same way slug/spec are.
    validatePathElement("mix[].family", entry.family);
    if (!Number.isInteger(entry.count) || entry.count < 1) {
      throw new Error("invalid mix[].count: must be a positive integer");
    }
  }
}

function validateArgs(a) {
  if (!a || typeof a !== "object") {
    throw new Error("invalid args: input is missing or not an object");
  }
  validatePathElement("slug", a.slug);
  validatePathElement("spec", a.spec);
  validateBeadId(a.bead_id);
  validateTarget(a.target);
  validateRound(a.round);
  validateMix(a.mix);
}

// ---------------------------------------------------------------------------
// Verdict shapes (the 110 contract, mirrored here for the `schema` option —
// this constrains an agent step's in-memory RETURN value only; it is not an
// on-disk-artifact guarantee, which is why `mindspec panel verify` still runs
// deterministic post-hoc validation in Step 5 below).

const VERDICT_SCHEMA = {
  type: "object",
  required: [
    "reviewer_id",
    "verdict",
    "confidence",
    "rationale",
    "concrete_changes_required",
    "findings",
  ],
  properties: {
    reviewer_id: { type: "string" },
    verdict: { type: "string", enum: ["APPROVE", "REQUEST_CHANGES", "REJECT"] },
    confidence: { type: "number" },
    rationale: { type: "string" },
    concrete_changes_required: { type: "array", items: { type: "string" } },
    findings: { type: "array", items: { type: "string" } },
    hard_block: { type: "boolean" },
  },
};

// The codex wrapper's own return shape: a status discriminant plus (when
// status is "ok") the transcribed verdict, plus the raw stdout it captured —
// the script has no filesystem access, so the retry prompt below needs the
// first attempt's raw output handed back to it as a return value, not read
// from the .codex.log the wrapper persisted to disk.
const CODEX_WRAPPER_SCHEMA = {
  type: "object",
  required: ["status"],
  properties: {
    status: { type: "string", enum: ["ok", "parse_failure", "quota_wall"] },
    rawStdout: { type: "string" },
    verdict: VERDICT_SCHEMA,
  },
};

// ---------------------------------------------------------------------------
// flattenMix — one {slotId, family, lens} descriptor per unit of mix[].count.
// slotId is drawn from the fixed R1, R2, ... enumeration (generated here from
// the loop index), never interpolated from args.
function flattenMix(mix, lenses) {
  const descriptors = [];
  let index = 0;
  for (const { family, count } of mix) {
    for (let i = 0; i < count; i++) {
      const slotId = `R${index + 1}`;
      const lens = Array.isArray(lenses) && lenses.length > 0 ? lenses[index % lenses.length] : undefined;
      descriptors.push({ slotId, family, lens });
      index++;
    }
  }
  return descriptors;
}

function lensLine(descriptor) {
  return descriptor.lens ? descriptor.lens : "(no specific lens assigned)";
}

// ---------------------------------------------------------------------------
// Step 3a — a claude slot: an agent() step that reads the BRIEF, renders a
// verdict, and writes it to the 110 contract path itself.
async function runClaudeSlot(descriptor, round, panelDir, reviewerId) {
  const verdictPath = `${panelDir}/${descriptor.slotId}-round-${round}.json`;
  await agent(
    `You are reviewer ${reviewerId} on this review panel. Read the BRIEF at ` +
      `${panelDir}/BRIEF.md and review it applying this lens: ${lensLine(descriptor)}\n\n` +
      "Render your verdict as a JSON object with these fields: reviewer_id, " +
      "verdict (one of APPROVE, REQUEST_CHANGES, REJECT), confidence, " +
      "rationale, concrete_changes_required (array of strings), findings " +
      `(array of strings), and optionally hard_block. Set reviewer_id to ` +
      `exactly "${reviewerId}". Write that exact JSON object to ${verdictPath} ` +
      "using your Write tool, then return the same object.",
    { schema: VERDICT_SCHEMA, label: descriptor.slotId },
  );
}

// ---------------------------------------------------------------------------
// Step 3b — a codex slot: a wrapper agent. Codex itself NEVER writes a file —
// neither the verdict nor the audit log — the wrapper writes both with its
// own Write tool, and the --sandbox read-only pin on CMD_CODEX_EXEC makes
// that a sandbox-enforced guarantee rather than only a prompt convention.
//
// `priorRawStdout` is undefined on the first attempt; on the re-prompt it
// carries the reviewer's own first-attempt raw stdout so the reviewer can
// re-serialize its OWN rendered output rather than reviewing again.
async function execCodexOnce(descriptor, round, panelDir, reviewerId, priorRawStdout) {
  const codexCmd = buildCommand(CMD_CODEX_EXEC);
  const verdictPath = `${panelDir}/${descriptor.slotId}-round-${round}.json`;
  const logPath = `${panelDir}/${descriptor.slotId}-round-${round}.codex.log`;

  const prompt =
    priorRawStdout === undefined
      ? `You are reviewer ${reviewerId} on this review panel. Read the BRIEF ` +
        `at ${panelDir}/BRIEF.md and review it applying this lens: ${lensLine(descriptor)}\n\n` +
        "Run exactly this command and do not modify it, feeding it the BRIEF " +
        `content and lens on stdin:\n\n${codexCmd}\n\n` +
        "This runs in a read-only sandbox, so it will never write any file " +
        "itself. Capture its COMPLETE, UNMODIFIED stdout. Using your own " +
        `Write tool, persist that raw text verbatim to ${logPath} — the audit ` +
        "log — before or as you transcribe it. Also return that same raw " +
        "text as rawStdout in your response, whatever the outcome below.\n\n" +
        "Then determine which of these applies:\n" +
        "- The stdout shows a usage-limit signal and contains NO verdict " +
        'content at all (a quota wall): return {status: "quota_wall", rawStdout}.\n' +
        "- The stdout contains EXACTLY ONE JSON object matching the verdict " +
        "shape (reviewer_id, verdict, confidence, rationale, " +
        "concrete_changes_required, findings, optional hard_block), with no " +
        "other JSON object and no surrounding narrative text: set its " +
        `reviewer_id to exactly "${reviewerId}", write that exact JSON object ` +
        `to ${verdictPath} using your Write tool, and return {status: "ok", ` +
        "rawStdout, verdict: <that object>}.\n" +
        "- Anything else (zero objects, more than one object, or a single " +
        "object plus surrounding narrative text all count as NOT accepted): " +
        'return {status: "parse_failure", rawStdout}.'
      : `You are the same reviewer ${reviewerId}. Your own previous rendered ` +
        `output was:\n\n${priorRawStdout}\n\n` +
        "That output either was not valid JSON, or did not contain exactly " +
        "one verdict object. Do NOT review again and do NOT change your " +
        "findings — re-emit the SAME verdict you already rendered above as a " +
        "single, clean, standalone JSON object with no surrounding text, " +
        `keeping reviewer_id exactly "${reviewerId}". This is a serialization ` +
        "retry, not a fresh review.\n\n" +
        "If you can recover and re-serialize your own prior verdict: write " +
        `that exact JSON object to ${verdictPath} using your Write tool, and ` +
        'return {status: "ok", verdict: <that object>}.\n' +
        "If you genuinely cannot recover a verdict from your own prior " +
        'output: return {status: "parse_failure"} — do not fabricate a new ' +
        "review.";

  return agent(prompt, { schema: CODEX_WRAPPER_SCHEMA, label: descriptor.slotId });
}

// The anti-laundering ladder (R3, R3b):
//   (a) parse failure on a rendered verdict -> re-prompt the SAME reviewer
//       exactly once, asking only for a re-serialize (never a fresh review,
//       never claude-sub);
//   (b) still unparseable after that single re-prompt -> fails CLOSED to a
//       MISSING verdict (returned as outcome "missing" here; never
//       substituted, never replaced by another reviewer's verdict);
//   (c) substitution is reserved EXCLUSIVELY for a quota wall in which no
//       verdict content was EVER rendered — a rendered-but-malformed verdict
//       (this ladder) is out of substitution's reach even if the retry
//       itself later reports a quota wall.
async function runCodexSlot(descriptor, round, panelDir) {
  const reviewerId = `${descriptor.slotId} codex`;

  const first = await execCodexOnce(descriptor, round, panelDir, reviewerId, undefined);

  if (first.status === "ok") {
    return { slotId: descriptor.slotId, outcome: "ok" };
  }
  if (first.status === "quota_wall") {
    // No verdict content was ever rendered — this slot IS eligible for
    // Step 4's substitution branch.
    return { slotId: descriptor.slotId, outcome: "quota_wall" };
  }

  // (a) Parse failure on a rendered verdict: re-prompt the same reviewer
  // exactly once, feeding back its own rendered output.
  const retry = await execCodexOnce(descriptor, round, panelDir, reviewerId, first.rawStdout ?? "");

  if (retry.status === "ok") {
    return { slotId: descriptor.slotId, outcome: "ok" };
  }

  // (b) Still unparseable — or the retry itself hit a quota wall — after the
  // single re-prompt: this slot already rendered content on its first
  // attempt, so it fails CLOSED to MISSING rather than being substituted.
  return { slotId: descriptor.slotId, outcome: "missing" };
}

async function runSlot(descriptor, round, panelDir) {
  if (descriptor.family === "codex") {
    return runCodexSlot(descriptor, round, panelDir);
  }
  if (descriptor.family === "claude") {
    const reviewerId = `${descriptor.slotId} claude`;
    await runClaudeSlot(descriptor, round, panelDir, reviewerId);
    return { slotId: descriptor.slotId, outcome: "ok" };
  }
  throw new Error(`unknown reviewer family for slot ${descriptor.slotId}: ${descriptor.family}`);
}

// ---------------------------------------------------------------------------
// Step 4 — quota-wall substitution: a deterministic branch driven by the
// wrapper's returned status, never a human judgment call. Applies ONLY to
// slots whose outcome is "quota_wall" (Step 3's ladder never routes a
// rendered-but-malformed verdict here).
async function substituteClaudeSlot(descriptor, round, panelDir) {
  const reviewerId = `${descriptor.slotId} claude-sub`;
  const verdictPath = `${panelDir}/${descriptor.slotId}-round-${round}.json`;
  await agent(
    `You are substituting for reviewer ${descriptor.slotId} because the ` +
      "codex reviewer originally assigned to this slot hit a usage-limit " +
      "quota wall before rendering any verdict content. Read the BRIEF at " +
      `${panelDir}/BRIEF.md and review it applying this lens: ${lensLine(descriptor)}\n\n` +
      "Render your verdict as a JSON object with these fields: reviewer_id, " +
      "verdict (one of APPROVE, REQUEST_CHANGES, REJECT), confidence, " +
      "rationale, concrete_changes_required (array of strings), findings " +
      `(array of strings), and optionally hard_block. Set reviewer_id to ` +
      `exactly "${reviewerId}" — the same slot id, substituted. Write that ` +
      `exact JSON object to ${verdictPath} using your Write tool, then ` +
      "return the same object.",
    { schema: VERDICT_SCHEMA, label: descriptor.slotId },
  );
}

// ===========================================================================
// The workflow body.

validateArgs(args);

const round = args.round;
const claudeSubOnQuota = args.claude_sub_on_quota === true; // fail-closed default

// Step 2 (R2) — registration through the create verb only. panel.json (round
// + reviewed_head_sha co-bumped by construction, expected_reviewers /
// approve_threshold stamped from the 109 resolvers) is written by the binary,
// never hand-typed here. `args.sha` is deliberately never read below — it is
// advisory-only display text, not the source of the recorded SHA.
const registerCmd = buildCommand(CMD_PANEL_CREATE, args.slug, args.spec, args.target, args.bead_id, round);
const registration = await agent(
  `Run exactly this command and do not modify it:\n\n${registerCmd}\n\n` +
    "This registers the review panel, writing panel.json and a BRIEF.md to a " +
    "directory that the command reports in its own stdout. Read that output " +
    "and find the directory path it reports — do not construct or guess the " +
    "path yourself, read it from the command's own output — and return it." +
    (args.sha
      ? ` (A hint sha of ${args.sha} was supplied; it is advisory only — the ` +
        "command above resolves the authoritative SHA itself, so ignore this " +
        "hint for anything other than display.)"
      : ""),
  {
    schema: {
      type: "object",
      required: ["panelDir"],
      properties: { panelDir: { type: "string" } },
    },
  },
);

// The single reported layout every later step derives its write paths from —
// no slot recomputes this independently from raw spec/slug.
const panelDir = registration.panelDir;

// Step 3 (R3, R3b) — fan out one agent step per mix slot via pipeline(), the
// documented concurrent fan-out primitive (bounded by the runtime's
// 16-concurrent-agent cap).
const descriptors = flattenMix(args.mix, args.lenses);
const slotResults = await pipeline(descriptors, (descriptor) => runSlot(descriptor, round, panelDir));

// Step 4 (R4) — quota-wall substitution over the fan-out results.
const finalResults = [];
for (const result of slotResults) {
  if (result.outcome !== "quota_wall") {
    finalResults.push(result);
    continue;
  }
  const descriptor = descriptors.find((d) => d.slotId === result.slotId);
  if (claudeSubOnQuota) {
    await substituteClaudeSlot(descriptor, round, panelDir);
    finalResults.push({ slotId: result.slotId, outcome: "substituted" });
  } else {
    // Leave the slot unfilled — no fabricated verdict, no silent skip. The
    // verify step below reports it as missing/incomplete and the gate Blocks.
    finalResults.push({ slotId: result.slotId, outcome: "missing" });
  }
}

// Step 5 (R5) — return verify + tally output verbatim. Never runs the
// lifecycle merge-terminal command, never consolidates, never authors a
// consolidated-round file, never mutates panel.json beyond what the create
// verb above wrote.
const verifyCmd = buildCommand(CMD_PANEL_VERIFY, args.slug);
const verifyResult = await agent(
  `Run exactly this command and do not modify it:\n\n${verifyCmd}\n\n` +
    "Return its complete stdout verbatim — do not summarize, reformat, or " +
    "paraphrase any part of it.",
  {
    schema: {
      type: "object",
      required: ["stdout"],
      properties: { stdout: { type: "string" } },
    },
  },
);

const tallyCmd = buildCommand(CMD_PANEL_TALLY, args.slug);
const tallyResult = await agent(
  `Run exactly this command and do not modify it:\n\n${tallyCmd}\n\n` +
    "Return its complete stdout verbatim — do not summarize, reformat, or " +
    "paraphrase any part of it.",
  {
    schema: {
      type: "object",
      required: ["stdout"],
      properties: { stdout: { type: "string" } },
    },
  },
);

// The single structured result: both CLI outputs carried through verbatim,
// plus the per-slot outcomes for the invoking skill's own bookkeeping.
return {
  panelDir,
  slotResults: finalResults,
  verifyOutput: verifyResult.stdout,
  tallyOutput: tallyResult.stdout,
};
