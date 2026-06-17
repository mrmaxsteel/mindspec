#!/usr/bin/env bash
# run_eval.sh — LLM-grill detection eval (spec 105, Req 14, AC1/AC6/AC7/AC8).
#
# Runs the ms-spec-grill prompt over each fixture N>=5 times via the pinned
# Claude Code CLI, then scores each run's findings against ground_truth.tsv with
# a DETERMINISTIC structured-anchor matcher (no LLM judging an LLM):
#   credit a row IFF a [CATEGORY] tag in the row category's equivalence family
#   appears in a finding line AND its `anchor` appears as a NORMALIZED substring
#   (case-fold + strip quotes/punct) of that SAME finding line. One-to-one: each
#   row credited at most once. recall = matched / M.
#
# ANTI-PARROT PRECISION GATE: an adversarial panel showed a degenerate "parrot"
# (tag EVERY fixture line [SEMANTIC]/[CONTRADICTION]) scored 15/15 recall with
# ZERO comprehension, because surplus findings used to neither credit nor
# penalize. We now ALSO compute precision = (distinct M-class rows matched) /
# (total M-class-TAGGED finding lines emitted) and gate PASS on BOTH recall >=
# ceil(0.9*M) AND precision >= a floor (0.50). A real grill emits ~M findings
# mostly matching → precision near 1.0; a parrot tags ~100 lines, only ~M match →
# precision ~0.14 → FAIL. A hermetic degenerate-parrot SELF-TEST (no claude)
# constructs that parrot and asserts it fails the gate — making "the eval is not
# hollow" an enforced, bead-blocking invariant.
#
#   Category families: the vagueness family {SEMANTIC, SYNONYM, EXACT_PHRASE} is
#   treated as interchangeable (the grill's own taxonomy blurs them — e.g. a
#   phrase can be both "vague" and on the EXACT_PHRASE blocklist — so the exact
#   sub-tag the model emits drifts run-to-run). CONTRADICTION, GROUNDING and
#   STRUCTURAL stay STRICT (no cross-family credit). The fixture-pinned anchor
#   still localizes the match to the exact planted span, so families never let an
#   unrelated finding credit a row.
#
# M is computed from the tracked TSV (SEMANTIC+SYNONYM+CONTRADICTION rows), never
# a frozen literal. Reports MIN and median recall over the N runs plus the
# deterministic baseline (0/M) for comparison; prints the held-out fixture's
# recall separately. Exits 0 only when min recall >= ceil(0.9*M).
#
# Reproducibility: the Claude Code CLI exposes NO --temperature flag (verified
# v2.1.178). Determinism is realized by the FIXED model pin + N-run MIN/median
# aggregation, NOT a temperature knob.
#
# PRECONDITION: `claude` must be installed AND authenticated. If it is not, this
# script SKIPS-with-notice (prints a message, exits 0) — it never hard-fails —
# so it is runnable on fresh machines / CI. The hermetic anchor self-check (C4)
# still runs in the skip path.
set -u

# --- pinned model id (FIXED full model id; NO temperature flag exists) ---------
MODEL_ID="${MS_GRILL_MODEL:-claude-opus-4-20250514}"
N_RUNS="${MS_GRILL_RUNS:-5}"

# --- precision floor (anti-parrot) --------------------------------------------
# A run PASSES only when recall >= ceil(0.9*M) AND precision >= this floor.
# 0.50 is generous: a real grill that finds a few EXTRA genuine defects still
# clears it (e.g. M+3 tagged lines all real -> precision 1.0; even M correct + a
# handful of spurious tags stays >0.5), yet a tag-everything parrot (~M of ~100
# tagged lines match -> precision ~0.14) fails hard. Stored as integer percent to
# keep the gate arithmetic in awk integer-safe.
PRECISION_FLOOR_PCT="${MS_GRILL_PRECISION_FLOOR_PCT:-50}"

# --- locate the tree relative to this script ----------------------------------
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIX_DIR="$HERE/fixtures"
GT="$HERE/ground_truth.tsv"
# the single source of truth for the grill prompt:
SKILL="$HERE/../../plugins/mindspec/skills/ms-spec-grill/SKILL.md"

fail() { echo "ERROR: $*" >&2; exit 1; }

[ -f "$GT" ] || fail "ground_truth.tsv not found at $GT"
[ -d "$FIX_DIR" ] || fail "fixtures dir not found at $FIX_DIR"

# map a TSV fixture key to its on-disk fixture filename (spec4 -> spec4-heldout).
fixture_file() {
  case "$1" in
    spec4) echo "$FIX_DIR/spec4-heldout.md" ;;
    *)     echo "$FIX_DIR/$1.md" ;;
  esac
}

# --- C4 anchor self-check (hermetic; runs even when claude is absent) ----------
echo "== anchor self-check (C4): every ground_truth anchor is a substring of its fixture =="
anchor_skew=0
while IFS=$'\t' read -r fx id _cat _catch anchor _problem; do
  [ "$fx" = "fixture" ] && continue
  [ -z "${fx:-}" ] && continue
  ff="$(fixture_file "$fx")"
  if [ ! -f "$ff" ]; then
    echo "  SKEW: $fx/$id -> fixture file missing ($ff)"
    anchor_skew=1
    continue
  fi
  if ! grep -F -q -- "$anchor" "$ff"; then
    echo "  SKEW: $fx/$id anchor not found in $(basename "$ff"): [$anchor]"
    anchor_skew=1
  fi
done < "$GT"
if [ "$anchor_skew" -ne 0 ]; then
  fail "anchor/fixture skew detected (see SKEW lines above) — fix ground_truth.tsv anchors"
fi
echo "  OK: all anchors are substrings of their own fixture."
echo

# --- compute M (SEMANTIC+SYNONYM+CONTRADICTION rows) from the TSV --------------
M="$(awk -F'\t' 'NR>1 && ($3=="SEMANTIC"||$3=="SYNONYM"||$3=="CONTRADICTION"){c++} END{print c+0}' "$GT")"
[ "$M" -gt 0 ] || fail "M computed as 0 — no SEMANTIC/SYNONYM/CONTRADICTION rows"
# ceil(0.9 * M) without bc:
THRESH="$(awk -v m="$M" 'BEGIN{ v=0.9*m; t=int(v); if (v>t) t++; print t }')"
echo "M (SEMANTIC+SYNONYM+CONTRADICTION rows) = $M ; pass threshold = ceil(0.9*M) = $THRESH"
echo "Deterministic baseline on these classes = 0/$M (det_detect.sh flags none of them)."
echo "Precision floor (anti-parrot) = 0.$(printf '%02d' "$PRECISION_FLOOR_PCT") ; PASS requires recall >= $THRESH AND precision >= floor."
echo

# score ONE run's findings file against the TSV; echo "<matched> <M> <mclass_lines>".
#   matched       = distinct M-class ground_truth rows credited (recall numerator)
#   M             = total M-class ground_truth rows (recall denominator)
#   mclass_lines  = count of finding LINES the grill emitted that bear an M-class
#                   tag (SEMANTIC/SYNONYM/EXACT_PHRASE/CONTRADICTION) — the
#                   PRECISION denominator. precision = matched / mclass_lines.
#
# PRECISION RATIONALE: the plan's original "surplus neither penalizes" rule
# protected a grill that finds EXTRA REAL defects (a few extra genuine findings
# barely move precision). But it also let a degenerate "parrot" spray an M-class
# tag on EVERY fixture line and trivially match every anchor (recall high, but
# only ~M of ~100 tagged lines actually match → precision ~0.14). A precision
# FLOOR threads both cases: a handful of extra real findings keep precision well
# above the floor, while tag-everything tanks it. Recall stays the PRIMARY metric;
# precision is the anti-parrot FLOOR.
#
# also writes per-category hit markers to $3 (a tmp file) for AC6/AC7/AC8 report.
score_run() {
  local findings="$1" gt="$2" hitfile="$3"
  awk -F'\t' -v FIND="$findings" -v HIT="$hitfile" '
    function norm(s){
      s=tolower(s);
      gsub(/["'"'"'`.,;:!?()\[\]{}*_\/\\]/,"",s);
      gsub(/[ \t\r\n]+/," ",s);
      return s;
    }
    BEGIN{
      # load + normalize finding lines
      nf=0;
      while ((getline line < FIND) > 0){ nfl[++nf]=norm(line); rawl[nf]=line; }
      close(FIND);
      # PRECISION denominator: count finding lines bearing an M-class tag
      # (the VAGUE family {SEMANTIC,SYNONYM,EXACT_PHRASE} plus CONTRADICTION).
      mclass_lines=0;
      for(j=1;j<=nf;j++){
        lr=tolower(rawl[j]);
        if (index(lr,"[semantic]")>0 || index(lr,"[synonym]")>0 ||
            index(lr,"[exact_phrase]")>0 || index(lr,"[contradiction]")>0)
          mclass_lines++;
      }
    }
    # category-tag equivalence families. The vagueness family {SEMANTIC, SYNONYM,
    # EXACT_PHRASE} is treated as interchangeable because the grill taxonomy blurs
    # them (e.g. "is robust"/"reasonable time" is both a vague phrase AND on the
    # EXACT_PHRASE blocklist) and the exact sub-tag the model emits drifts run to
    # run. CONTRADICTION, GROUNDING and STRUCTURAL are distinct classes and stay
    # STRICT (no cross-family credit). The anchor (a fixture substring) still pins
    # the match to the right span, so a family does not let an unrelated finding
    # credit a row.
    function family(c){
      if (c=="SEMANTIC"||c=="SYNONYM"||c=="EXACT_PHRASE") return "VAGUE";
      return c;
    }
    function line_has_family(rawline, fam){
      if (fam=="VAGUE") {
        return (index(tolower(rawline),"[semantic]")>0 ||
                index(tolower(rawline),"[synonym]")>0 ||
                index(tolower(rawline),"[exact_phrase]")>0);
      }
      return index(tolower(rawline), "["tolower(fam)"]") > 0;
    }
    NR==1{next}
    $1==""{next}
    {
      fx=$1; id=$2; cat=$3; anchor=$5;
      fam=family(cat);
      na=norm(anchor);
      matched=0;
      for(i=1;i<=nf;i++){
        if(used[i]) continue;
        # a tag in the row-category family present in the raw finding line AND
        # the normalized anchor is a substring of the normalized finding line.
        if (line_has_family(rawl[i], fam) && index(nfl[i], na) > 0){
          used[i]=1; matched=1;
          print cat" "fx"/"id >> HIT;
          break;
        }
      }
      if (cat=="SEMANTIC"||cat=="SYNONYM"||cat=="CONTRADICTION"){
        Mrows++;
        if (matched) hit++;
      }
    }
    END{ print hit+0, Mrows+0, mclass_lines+0 }
  ' "$gt"
}

# emit "<recall_pass> <prec_pass>" (1/0 each) given matched, M-rows, mclass_lines,
# the recall threshold, and the precision floor (as integer percent, e.g. 50).
# precision = matched / mclass_lines; if no M-class lines were emitted at all,
# precision is treated as 0 (a grill that emits zero findings cannot pass).
gate_parts() {
  local matched="$1" mrows="$2" mlines="$3" thresh="$4" floorpct="$5"
  awk -v m="$matched" -v M="$mrows" -v ml="$mlines" -v t="$thresh" -v fp="$floorpct" 'BEGIN{
    rp = (m >= t) ? 1 : 0;
    prec = (ml > 0) ? (m / ml) : 0;
    pp = (prec*100 >= fp) ? 1 : 0;
    print rp, pp;
  }'
}

# pretty-print precision as a 2-decimal fraction for reports.
prec_str() {
  awk -v m="$1" -v ml="$2" 'BEGIN{ if(ml>0) printf "%.2f", m/ml; else printf "0.00"; }'
}

# --- HERMETIC degenerate-parrot SELF-TEST (no claude; bead-blocking) -----------
# Deterministically construct the "parrot" output: for EVERY line of EVERY
# fixture, emit  [SEMANTIC] [CONTRADICTION] "<that exact line>" — parrot.
# This is the pathological grill the adversarial panel found scored 15/15 under
# the old recall-only gate (it sprays M-class tags on all ~100 lines, so every
# anchor trivially matches → recall is maxed, ZERO comprehension). Feed it
# through the SAME score_run/gate logic and ASSERT it FAILS the PASS condition
# (its precision ≈ matched/total-tagged-lines must fall below the floor). If the
# parrot ever PASSES, the eval is hollow → HARD FAIL (exit non-zero) so a future
# matcher change that reopens the hole breaks this test loudly.
echo "== degenerate-parrot self-test (hermetic; runs without claude) =="
PTMP="$(mktemp -d)"
parrot_findings="$PTMP/parrot.findings"
: > "$parrot_findings"
for ff in "$FIX_DIR"/*.md; do
  while IFS= read -r line || [ -n "$line" ]; do
    printf '[SEMANTIC] [CONTRADICTION] "%s" — parrot\n' "$line" >> "$parrot_findings"
  done < "$ff"
done
parrot_hits="$PTMP/parrot.hits"
: > "$parrot_hits"
read -r p_matched p_mrows p_mlines < <(score_run "$parrot_findings" "$GT" "$parrot_hits")
read -r p_rp p_pp < <(gate_parts "$p_matched" "$p_mrows" "$p_mlines" "$THRESH" "$PRECISION_FLOOR_PCT")
p_prec="$(prec_str "$p_matched" "$p_mlines")"
if [ "$p_rp" -eq 1 ] && [ "$p_pp" -eq 1 ]; then
  echo "PARROT SELF-TEST: recall=$p_matched/$p_mrows precision=$p_prec ($p_mlines tagged lines) → PASSED the gate"
  echo "  *** EVAL IS HOLLOW: the parrot beat the gate. Aborting. ***" >&2
  rm -rf "$PTMP"
  exit 3
fi
echo "PARROT SELF-TEST: recall=$p_matched/$p_mrows precision=$p_prec ($p_mlines M-class-tagged lines) → FAIL (below gate, floor=0.$(printf '%02d' "$PRECISION_FLOOR_PCT")) ✓"
echo "  (parrot recall may be high, but precision $p_prec < floor → the gate correctly rejects tag-everything.)"
rm -rf "$PTMP"
echo

# --- claude precondition: SKIP-with-notice (exit 0) if absent/unauthenticated --
# (Runs AFTER the hermetic anchor + parrot self-tests so those gate even on a
# fresh machine with no `claude`.)
if ! command -v claude >/dev/null 2>&1; then
  echo "SKIP: \`claude\` CLI not installed. Anchor + parrot self-tests passed; LLM recall not measured."
  echo "      (This is an ADVISORY eval — skip is not a failure.)"
  exit 0
fi
if [ -z "${ANTHROPIC_API_KEY:-}" ] && [ ! -d "$HOME/.claude" ]; then
  echo "SKIP: \`claude\` present but no auth detected (no ANTHROPIC_API_KEY and no ~/.claude)."
  echo "      Anchor + parrot self-tests passed; LLM recall not measured. (ADVISORY — skip is not a failure.)"
  exit 0
fi
[ -f "$SKILL" ] || fail "grill SKILL.md not found at $SKILL (Bead 1 must merge first)"

GRILL_PROMPT="$(cat "$SKILL")"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

declare -a RECALLS
declare -a PREC_PCTS   # per-run precision as integer percent (for MIN aggregation)
ALL_HITS="$TMP/all_hits.txt"
: > "$ALL_HITS"

n_fixtures="$(find "$FIX_DIR" -maxdepth 1 -name '*.md' | wc -l | tr -d ' ')"
echo "== running grill: model=$MODEL_ID, N=$N_RUNS runs over $n_fixtures fixtures =="
for run in $(seq 1 "$N_RUNS"); do
  run_findings="$TMP/run${run}.findings"
  : > "$run_findings"
  for ff in "$FIX_DIR"/*.md; do
    prompt="$GRILL_PROMPT

---
Grill the following draft spec. Emit ONLY the tagged finding lines per your
output contract — one finding per line as: [CATEGORY] \"verbatim span\" — critique.
Do not ask questions; this is a non-interactive batch evaluation.

=== BEGIN SPEC: $(basename "$ff") ===
$(cat "$ff")
=== END SPEC ===
"
    # capture findings; tolerate a failed call (counts as zero findings for that fixture).
    if out="$(printf '%s' "$prompt" | claude -p --model "$MODEL_ID" 2>/dev/null)"; then
      printf '%s\n' "$out" >> "$run_findings"
    else
      echo "  (run $run, $(basename "$ff")): claude call failed — counted as no findings" >&2
    fi
  done

  hitfile="$TMP/run${run}.hits"
  : > "$hitfile"
  read -r matched mrows mlines < <(score_run "$run_findings" "$GT" "$hitfile")
  cat "$hitfile" >> "$ALL_HITS"
  RECALLS[run]="$matched"
  prec="$(prec_str "$matched" "$mlines")"
  PREC_PCTS[run]="$(awk -v m="$matched" -v ml="$mlines" 'BEGIN{ if(ml>0) printf "%d", (m*100)/ml; else print 0 }')"

  # held-out fixture (spec4) recall this run
  ho_total="$(awk -F'\t' 'NR>1 && $1=="spec4" && ($3=="SEMANTIC"||$3=="SYNONYM"||$3=="CONTRADICTION"){c++} END{print c+0}' "$GT")"
  # count only M-class (SEMANTIC/SYNONYM/CONTRADICTION) spec4 hits, to match ho_total.
  ho_hit="$(awk '$2 ~ /^spec4\// && ($1=="SEMANTIC"||$1=="SYNONYM"||$1=="CONTRADICTION"){c++} END{print c+0}' "$hitfile")"
  echo "  run $run: recall = $matched/$mrows ; precision = $prec ($matched/$mlines M-tagged lines) ; held-out(spec4) = $ho_hit/$ho_total"
done

# --- aggregate MIN + median over the N runs -----------------------------------
sorted="$(printf '%s\n' "${RECALLS[@]}" | sort -n)"
MINR="$(printf '%s\n' "$sorted" | head -1)"
MEDR="$(printf '%s\n' "$sorted" | awk '{a[NR]=$1} END{ if(NR%2) print a[(NR+1)/2]; else print int((a[NR/2]+a[NR/2+1])/2) }')"

# MIN precision (integer percent) over the N runs — the anti-parrot gate.
psorted="$(printf '%s\n' "${PREC_PCTS[@]}" | sort -n)"
MINP_PCT="$(printf '%s\n' "$psorted" | head -1)"
MINP_STR="$(awk -v p="$MINP_PCT" 'BEGIN{ printf "%.2f", p/100 }')"
FLOOR_STR="$(awk -v p="$PRECISION_FLOOR_PCT" 'BEGIN{ printf "%.2f", p/100 }')"

echo
echo "== aggregate over $N_RUNS runs =="
echo "  recalls (raw): ${RECALLS[*]}"
echo "  precisions% (raw): ${PREC_PCTS[*]}"
echo "  MIN recall    = $MINR/$M"
echo "  MEDIAN recall = $MEDR/$M"
echo "  MIN precision = $MINP_STR  (floor $FLOOR_STR)"
echo "  pass threshold = $THRESH/$M  (ceil 0.9*M)"
echo "  deterministic baseline = 0/$M"
echo
echo "== behavioral spot-checks (AC6 grounding / AC7 invented-domain / AC8 scenario), union over runs =="
if grep -q 'GROUNDING' "$ALL_HITS"; then echo "  AC6/AC7 GROUNDING (invented domain) detected: YES"; else echo "  AC6/AC7 GROUNDING detected: no"; fi
if grep -q 'CONTRADICTION' "$ALL_HITS"; then echo "  CONTRADICTION detected: YES"; else echo "  CONTRADICTION detected: no"; fi

echo
# PASS requires BOTH: recall >= threshold (PRIMARY) AND precision >= floor (anti-parrot).
recall_ok=0; [ "$MINR" -ge "$THRESH" ] && recall_ok=1
prec_ok=0;   [ "$MINP_PCT" -ge "$PRECISION_FLOOR_PCT" ] && prec_ok=1
if [ "$recall_ok" -eq 1 ] && [ "$prec_ok" -eq 1 ]; then
  echo "PASS: min recall $MINR >= threshold $THRESH (of M=$M) AND min precision $MINP_STR >= floor $FLOOR_STR; beats det baseline 0/$M."
  exit 0
else
  reason=""
  [ "$recall_ok" -eq 0 ] && reason="min recall $MINR < threshold $THRESH"
  [ "$prec_ok" -eq 0 ] && reason="${reason:+$reason; }min precision $MINP_STR < floor $FLOOR_STR (anti-parrot)"
  echo "FAIL: $reason (of M=$M)."
  exit 1
fi
