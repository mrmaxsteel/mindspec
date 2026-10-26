// Coordinate reviewer slots through the panel commands.
const ALLOWED_CLI = [
  "mindspec panel create",
  "mindspec panel verify",
  "mindspec panel tally",
  "mindspec panel launch",
  "mindspec panel checkout",
  "mindspec panel prompt",
  "mindspec panel submit",
  "mindspec panel recover",
];
const [CREATE, VERIFY, TALLY, LAUNCH, CHECKOUT, PROMPT, SUBMIT, RECOVER] = ALLOWED_CLI;

function quote(value) {
  if (typeof value !== "string" || !value || value.startsWith("-") || /[\x00-\x1f\x7f]/.test(value)) {
    throw new Error("invalid command value");
  }
  return "'" + value.replace(/'/g, "'\"'\"'") + "'";
}

function absoluteBriefFile(value) {
  return typeof value === "string" && (value.startsWith("/") || /^[A-Za-z]:[\\/]/.test(value));
}

function buildCommand(verb, value = {}) {
  if (!ALLOWED_CLI.includes(verb)) throw new Error("command is not allowed");
  if (verb === SUBMIT) {
    if (!value.slotDir || !value.slotDir.startsWith("/") || !Number.isInteger(value.attempt) || value.attempt < 1) {
      throw new Error("submission needs an absolute slot directory and an attempt");
    }
    return verb + " --file " + quote(value.slotDir + "/attempt-" + value.attempt + "/verdict.json");
  }
  let command = verb + " " + quote(args.slug);
  if (args.spec) command += " --spec " + quote(args.spec);
  switch (verb) {
    case CREATE:
      if (!absoluteBriefFile(args.brief_file)) throw new Error("brief_file must be an absolute path");
      command += " --brief-file " + quote(args.brief_file);
      if (args.spec && args.gate === "adhoc") throw new Error("gate conflicts with spec");
      if (args.bead_id && args.gate && args.gate !== "bead") throw new Error("gate conflicts with bead_id");
      if (!args.bead_id) {
        if (args.gate) command += " --gate " + quote(args.gate);
        else if (!args.spec) command += " --gate adhoc";
      }
      command += " --target " + quote(args.target) + " --round " + args.round;
      if (args.bead_id) command += " --gate bead --bead " + quote(args.bead_id);
      break;
    case VERIFY: command += " --json"; break;
    case TALLY: break;
    case LAUNCH:
      command += " " + quote(value.slot);
      break;
    case CHECKOUT:
      command += " --slot " + quote(value.slot) + " --dispatch";
      break;
    case PROMPT: command += " --slot " + quote(value.slot); break;
    case RECOVER: command += " " + quote(value.slot) + " --reason " + quote("restore recorded reviewer output"); break;
    default: throw new Error("unknown command");
  }
  return command;
}

async function cli(verb, value) {
  return agent("Run exactly this command. Return its stdout, stderr and exit code.\n" + buildCommand(verb, value), {
    label: "panel-command",
    schema: { type: "object", required: ["stdout", "stderr", "exit"], properties: { stdout: { type: "string" }, stderr: { type: "string" }, exit: { type: "integer" } } },
  });
}

// Bound after escaping so control characters cannot expand the diagnostic.
function diagnostic(value) {
  const text = typeof value === "string" ? value : JSON.stringify(value);
  return JSON.stringify(text ?? "").slice(0, 200);
}

function failure(verb, result) {
  return "panel " + verb + " failed (exit " + result.exit + "): " + diagnostic(result.stderr || result.stdout);
}

async function roster() {
  const result = await cli(VERIFY);
  let slots;
  try {
    slots = JSON.parse(result.stdout);
  } catch {
    throw new Error("panel verify returned non-JSON stdout (exit " + result.exit + "); stdout: " + diagnostic(result.stdout) + (result.stderr ? "; stderr: " + diagnostic(result.stderr) : ""));
  }
  if (slots && typeof slots === "object" && !Array.isArray(slots) && "error" in slots) {
    throw new Error("panel verify: " + diagnostic(slots.error));
  }
  if (!Array.isArray(slots)) throw new Error("panel verify did not return a slot roster (exit " + result.exit + ")");
  return { slots, result };
}

async function dispatch(slot) {
  const checkout = await cli(CHECKOUT, { slot: slot.slot });
  if (checkout.exit !== 0) throw new Error(failure("checkout", checkout));
  const slotDir = checkout.stdout.trim();
  const prompt = await cli(PROMPT, { slot: slot.slot });
  if (prompt.exit !== 0) throw new Error(failure("prompt", prompt));
  const attempts = [];
  for (const line of prompt.stdout.split("\n")) {
    const match = line.match(/\/attempt-(\d+)\/verdict\.json/);
    if (!match) continue;
    const attempt = Number(match[1]);
    if (line === buildCommand(SUBMIT, { slotDir, attempt })) attempts.push(attempt);
  }
  if (attempts.length !== 1) throw new Error("slot prompt needs one bound submission command");
  const attempt = attempts[0];
  const submit = buildCommand(SUBMIT, { slotDir, attempt });
  const options = { label: slot.slot };
  if (slot.model !== slot.family) options.model = slot.model;
  try {
    await agent(prompt.stdout + "\nSave your verdict JSON with shell redirection to the absolute file in this command. " +
      "Do not use the Write tool on a verdict path. Submit through this exact command:\n" + submit, {
      ...options,
    });
  } catch (error) {
    // Reviewer failure follows the same verify-and-retry path as a silent return.
    return String(error?.message ?? error);
  }
}

async function runSlot(initial) {
  let slot = initial;
  let acted = false;
  let retried = initial.status !== "missing" && !initial.handoff_pending;
  let reviewerError;
  while (slot.status !== "final") {
    if (slot.status === "running" && !acted) return slot;
    if (slot.kind === "process") {
      if (slot.status === "running" || slot.status === "quota_wall") return slot;
      if (slot.status === "damaged") {
        const recovered = await cli(RECOVER, { slot: slot.slot });
        if (recovered.exit !== 0) return { ...slot, error: failure("recover", recovered) };
        try {
          const next = (await roster()).slots.find(s => s.slot === slot.slot);
          return next || { ...slot, error: "recovered slot disappeared from verify" };
        } catch (error) {
          return { ...slot, error: String(error?.message ?? error) };
        }
      }
      if (!["missing", "draft", "unsubmitted", "void", "launch_failed", "malformed"].includes(slot.status)) return slot;
      if (acted) { if (retried) return slot; retried = true; }
      // The command agent waits for this background workflow branch.
      const launched = await cli(LAUNCH, { slot: slot.slot });
      if (launched.exit !== 0) {
        // A quota handoff exits nonzero but verify records its next runner.
        let next;
        try { next = (await roster()).slots.find(s => s.slot === slot.slot); } catch {}
        if (next?.handoff_pending && next.kind === "harness") {
          slot = next; acted = false; retried = false;
          continue;
        }
        const error = failure("launch", launched);
        if (next?.kind === "process" && ["draft", "unsubmitted", "void", "launch_failed", "malformed"].includes(next.status)) {
          slot = { ...next, error }; acted = true;
          continue;
        }
        return { ...(next || slot), error };
      }
    } else if (slot.kind === "harness") {
      if (slot.status === "damaged" || slot.status === "quota_wall" && !slot.handoff_pending) return slot;
      if (!["missing", "running", "draft", "unsubmitted", "void", "launch_failed", "malformed", "quota_wall"].includes(slot.status)) return slot;
      if (acted && retried) return slot;
      try {
        const error = await dispatch(slot);
        if (error !== undefined) reviewerError = error;
      } catch (error) {
        return { ...slot, error: String(error?.message ?? error) };
      }
      if (acted) retried = true;
    } else {
      throw new Error("unknown runner kind");
    }
    acted = true;
    let next;
    try {
      next = (await roster()).slots.find(s => s.slot === slot.slot);
      if (!next) throw new Error("slot disappeared from verify");
    } catch (error) {
      return { ...slot, error: String(error?.message ?? error) };
    }
    // A quota handoff is the first harness dispatch, not a harness retry.
    if (next.kind !== slot.kind) { acted = false; retried = false; }
    slot = next.status !== "final" && reviewerError !== undefined ? { ...next, error: reviewerError } : next;
  }
  return slot;
}

if (!args || !Number.isInteger(args.round) || args.round < 1) throw new Error("round must be positive");
quote(args.slug);
quote(args.target);
if (!absoluteBriefFile(args.brief_file)) throw new Error("brief_file must be an absolute path");
quote(args.brief_file);
const created = await cli(CREATE);
if (created.exit !== 0) return { createOutput: failure("create", created) };
const initial = await roster();
const slotResults = await pipeline(initial.slots, runSlot);
const final = await roster();
let tallyOutput = "";
if (final.result.exit === 0) tallyOutput = (await cli(TALLY)).stdout;
return { slotResults, verifyOutput: final.result.stdout, tallyOutput };
