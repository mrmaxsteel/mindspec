// docs_truth_registry_test.go — mindspec-6td7 (W0 Bead 7): registry
// hardening. Three findings from the W0 re-panel, all orchestrator-
// reproduced, all about project-docs/claims-registry.yaml's OWN
// contract rather than the doc-scanning rules R1/R2/R3 already
// enforce (docs_truth_test.go, docs_truth_docs_test.go):
//
//   - A1 (F2-r2-1, token-widening smuggle): appending a token to an
//     EXISTING claim is a one-line grant of legitimacy — the smuggled
//     token silently inherits that claim's real owner/becomes_true_when
//     and starts passing R1/R3's token-match exemption. Closed here by
//     a golden-token ratchet (goldenClaimTokens /
//     TestClaimsTokensMatchGolden), the same content-anchored,
//     audited-table idiom knownDocsTruthExceptions
//     (docs_truth_test.go) already uses: any edit to a claim's token
//     set fails until this Go-side table is updated in the same
//     change, in both directions (a changed/added claim with no
//     matching golden entry, AND a golden entry whose claim no longer
//     exists, both fail).
//
//   - F2-3 (clause 4 unenforceable): claims-registry.yaml's header
//     asserted the lint forces registry deletion when the owning work
//     lands. It didn't — nothing detected that a claim's referent now
//     resolves for real. checkClaimsLanded below makes this true for
//     the checkable subset: a claim token shaped like a `mindspec ...`
//     invocation or a `/ms-*` skill reference is checked against the
//     exact same ground truth R1/R2 already resolve against (the real
//     cmdRoot / skillUniverse); if it now resolves, the entry should
//     have been deleted and the lint fails. A token shaped like a bare
//     config-key name (loop-governance's gate_authority/handoff_log/
//     max_beads_per_wake) asserts a BEHAVIOR change, not existence —
//     "the binary reads loop.* to change behavior" — which R1/R2's
//     existence-resolution can't observe, so that class is NOT
//     mechanically landed-checkable this way. The registry header is
//     rewritten to say exactly this (not a blanket claim), because
//     overclaiming uniform enforcement here would just be the same
//     defect class in different clothing.
//
//     Final-gate finding L4-FINAL-3 sharpened this further: mindspec-
//     invocation-shaped and /ms-*-shaped tokens are not UNIFORMLY
//     existence-checkable either — loop-status's becomes_true_when
//     demands `mindspec loop status` report four specific runtime
//     states, and fix-cycle's demands `/ms-fix-cycle` run its lane
//     end-to-end, neither of which mere resolution/existence proves. A
//     placeholder verb or a stub SKILL.md would otherwise force
//     deletion of a truthful planned label before the real behavior
//     exists — the exact absent-vs-inert distinction this file's own
//     header says existence checks cannot decide, reintroduced as an
//     enforcement claim. claimLandedRequiresBehavior excludes both
//     real ids from mechanical enforcement until a claim-specific
//     behavioral oracle is written; deletion for them stays
//     review-driven in the meantime.
//
//   - F2-4 (dead fields): owner/locations/becomes_true_when were
//     parsed (owner, locations) or not even parsed (becomes_true_when)
//     but never READ by anything. checkClaimFieldsValid below reads
//     and validates all three: owner and becomes_true_when must be
//     non-empty, and every locations entry must resolve to a scanned
//     file that actually carries a marker for that claim's id.
package lint

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// --- A1: golden claims ratchet -----------------------------------------

// goldenClaimTokens is the audited, exact token set for every claim
// currently registered in project-docs/claims-registry.yaml, keyed by
// claim id. TestClaimsTokensMatchGolden fails on ANY difference from
// the live registry — added, removed, reordered tokens, or a whole
// new/missing claim id — until this table is edited in the same
// change. That forces a token-set edit to show up as a reviewable Go
// diff (this is the point: a one-line YAML addition alone is not
// enough to pass), rather than as invisible YAML growth that silently
// inherits an existing claim's owner and marker exemption.
var goldenClaimTokens = map[string][]string{
	"loop-status":     {"mindspec loop status"},
	"loop-governance": {"gate_authority", "handoff_log", "max_beads_per_wake"},
	"fix-cycle":       {"/ms-fix-cycle"},
}

// checkClaimTokensAgainstGolden is the golden-token ratchet's actual
// enforcement, extracted to a pure function (final-gate finding L1-4):
// before this extraction, the comparison and reporting lived inline in
// TestClaimsTokensMatchGolden, and the file's own "RED-on-inject
// regression fixture" (TestClaimsTokensMatchGolden_DetectsWidening)
// never called it — it asserted only that stringSlicesEqual returns
// false for two different slices, which pins a two-line helper, not
// the ratchet. A mutation that disabled the real comparison (e.g. `if
// false && !stringSlicesEqual(...)`) left the whole package green.
// Covers all three arms: a claim present in reg but absent from
// golden (needs a new audited entry), a claim whose live tokens
// differ from golden (the widening/narrowing smuggle), and a golden
// entry whose claim no longer exists in reg (stale golden entry).
func checkClaimTokensAgainstGolden(reg *claimsRegistryDoc, golden map[string][]string) []truthFinding {
	var out []truthFinding

	seen := make(map[string]bool, len(golden))
	ids := make([]string, 0, len(reg.Claims))
	for id := range reg.Claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		seen[id] = true
		g, ok := golden[id]
		if !ok {
			out = append(out, truthFinding{File: "project-docs/claims-registry.yaml", Text: "claim `" + id + "`", Why: "has no entry in goldenClaimTokens -- add one, auditing its exact token set, in this same change"})
			continue
		}
		if !stringSlicesEqual(reg.Claims[id].Tokens, g) {
			out = append(out, truthFinding{File: "project-docs/claims-registry.yaml", Text: "claim `" + id + "`", Why: fmt.Sprintf("tokens changed: registry has %v, golden has %v -- if this edit is audited, update goldenClaimTokens in this SAME change; if you did not make this edit, this is exactly the token-widening smuggle this ratchet exists to catch", reg.Claims[id].Tokens, g)})
		}
	}
	goldenIDs := make([]string, 0, len(golden))
	for id := range golden {
		goldenIDs = append(goldenIDs, id)
	}
	sort.Strings(goldenIDs)
	for _, id := range goldenIDs {
		if !seen[id] {
			out = append(out, truthFinding{File: "project-docs/claims-registry.yaml", Text: "claim `" + id + "`", Why: "golden entry no longer exists in claims-registry.yaml -- stale golden entry, remove it"})
		}
	}
	return out
}

func TestClaimsTokensMatchGolden(t *testing.T) {
	findings := checkClaimTokensAgainstGolden(sharedRegistry(t), goldenClaimTokens)
	if len(findings) != 0 {
		var b []string
		for _, f := range findings {
			b = append(b, f.String())
		}
		t.Fatalf("expected zero golden-token-ratchet findings for the real registry, got:\n  %s", joinLines(b))
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestClaimsTokensMatchGolden_DetectsWidening is the RED-on-inject
// regression fixture for A1/L1-4: simulates exactly the reported
// exploit (appending a fictional token to an existing claim's tokens)
// against a synthetic registry + golden table pair, independent of the
// real files, so the assertion is pinned even if the real registry
// changes shape later — and, unlike the pre-L1-4 version, actually
// CALLS checkClaimTokensAgainstGolden rather than only exercising
// stringSlicesEqual directly.
func TestClaimsTokensMatchGolden_DetectsWidening(t *testing.T) {
	golden := map[string][]string{"loop-status": {"mindspec loop status"}}
	widened := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"loop-status": {Tokens: []string{"mindspec loop status", "mindspec telepathy sync --brainwave"}},
	}}
	findings := checkClaimTokensAgainstGolden(widened, golden)
	requireFindingContains(t, findings, "claim `loop-status`", "the token-widening smuggle this ratchet exists to catch")
}

// TestClaimsTokensMatchGolden_DetectsStaleGoldenEntry is the reverse
// arm (L1-4's required "reverse case for the stale-golden arm"): a
// golden entry whose claim id no longer exists in the live registry
// must fail too — an entry a claim's deletion should have removed.
func TestClaimsTokensMatchGolden_DetectsStaleGoldenEntry(t *testing.T) {
	golden := map[string][]string{"loop-status": {"mindspec loop status"}, "long-deleted-claim": {"mindspec ghost"}}
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"loop-status": {Tokens: []string{"mindspec loop status"}},
	}}
	findings := checkClaimTokensAgainstGolden(reg, golden)
	requireFindingContains(t, findings, "claim `long-deleted-claim`", "stale golden entry")
}

// TestClaimsTokensMatchGolden_DetectsMissingGoldenEntry is the third
// arm: a claim present in the live registry with no golden entry at
// all must fail, prompting a human to add and audit one.
func TestClaimsTokensMatchGolden_DetectsMissingGoldenEntry(t *testing.T) {
	golden := map[string][]string{}
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"brand-new-claim": {Tokens: []string{"mindspec brand-new-verb"}},
	}}
	findings := checkClaimTokensAgainstGolden(reg, golden)
	requireFindingContains(t, findings, "claim `brand-new-claim`", "no entry in goldenClaimTokens")
}

// --- F2-3 / clause 4: has a claim's referent landed? --------------------

// claimLandedRequiresBehavior lists claim ids whose becomes_true_when
// demands more than existence — reporting specific runtime state
// (loop-status: "reports open panels, rounds, budget consumed, and
// halt state"), or a full end-to-end lane (fix-cycle: "runs the
// discover -> reproduce -> one-commit patch -> panel -> PR lane
// end-to-end") — that checkClaimsLanded's existence-only ground truth
// (cmdRoot.resolve / a skill-map lookup) structurally cannot observe
// (final-gate finding L4-FINAL-3). Without this exclusion, a
// placeholder Cobra command that merely resolves (but does not yet
// report the four states), or a stub SKILL.md that merely exists (but
// does not yet run the lane), would be reported "landed, delete" —
// forcing removal of a truthful planned label while the underlying
// capability is still incomplete, exactly the absent-vs-inert
// distinction this registry's own header already says existence
// checks cannot decide. Per L4-FINAL-3's own suggested fallback,
// deletion for these ids stays REVIEW-DRIVEN (a human reads the
// becomes_true_when text and judges it, same as before
// checkClaimsLanded existed) until a genuine per-claim behavioral
// oracle is written. checkClaimsLanded's existence check remains live
// for any OTHER, future claim whose becomes_true_when is satisfied by
// existence alone (see TestClaimsLanded_VerbNowResolvingFails, which
// uses a non-excluded synthetic id to prove the general mechanism
// still works).
var claimLandedRequiresBehavior = map[string]bool{
	"loop-status": true,
	"fix-cycle":   true,
}

// checkClaimsLanded enforces clause 4 for the checkable subset of
// claims: a token shaped like a `mindspec ...` invocation is resolved
// against cmdRoot (the same real-binary ground truth R1 uses); a
// token shaped like a `/name` skill reference is looked up in skills
// (the same ground truth R2 uses). If either now resolves/exists AND
// the claim id is not in claimLandedRequiresBehavior, the claim's
// referent has landed for real and the entry should have been deleted
// in the same change that landed it.
func checkClaimsLanded(reg *claimsRegistryDoc, cmdRoot *cmdNode, skills skillUniverse) []truthFinding {
	var out []truthFinding

	ids := make([]string, 0, len(reg.Claims))
	for id := range reg.Claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		if claimLandedRequiresBehavior[id] {
			continue
		}
		for _, tok := range reg.Claims[id].Tokens {
			switch {
			case tok == "mindspec" || strings.HasPrefix(tok, "mindspec "):
				if res := cmdRoot.resolve(tokenizeInvocation(tok)); res.Resolved {
					out = append(out, truthFinding{
						File: "project-docs/claims-registry.yaml",
						Line: 0,
						Text: "claim `" + id + "`",
						Why:  fmt.Sprintf("token %q now resolves against the real command tree -- the referent has landed; delete this entry and its markers (clause 4)", tok),
					})
				}
			case strings.HasPrefix(tok, "/"):
				name := strings.TrimPrefix(tok, "/")
				if _, ok := skills[name]; ok {
					out = append(out, truthFinding{
						File: "project-docs/claims-registry.yaml",
						Line: 0,
						Text: "claim `" + id + "`",
						Why:  fmt.Sprintf("token %q now resolves against the real skill/workflow universe -- the referent has landed; delete this entry and its markers (clause 4)", tok),
					})
				}
			}
		}
	}
	return out
}

// TestClaimsLanded_RealRegistryNotYetLanded is the negative sanity
// check: none of the three real, currently-registered claims should
// resolve yet (if one did, checkR1/checkR2 would no longer even be
// finding it unresolved in the first place, and TestDocsTruthReal
// would need a new exception, not a marker).
func TestClaimsLanded_RealRegistryNotYetLanded(t *testing.T) {
	findings := checkClaimsLanded(sharedRegistry(t), sharedCmdRoot(t), sharedSkills(t))
	if len(findings) != 0 {
		t.Fatalf("expected zero landed-claim findings against the real, unlanded registry, got: %v", findings)
	}
}

// TestClaimsLanded_SkillNowExistingFails is the RED-on-inject
// regression fixture for F2-3, reproducing the exact exploit shape
// named in the bead brief: "creating a claimed skill's SKILL.md with
// markers and registry untouched leaves the lint GREEN" — for a claim
// id whose becomes_true_when IS satisfied by mere existence (unlike
// the real fix-cycle claim; see
// TestClaimsLanded_BehavioralOnlyClaimNotForcedByExistence below for
// why THAT one must NOT fire on existence alone). A synthetic
// skillUniverse standing in for a landed skill, plus a synthetic
// registry entry not in claimLandedRequiresBehavior, must fail.
func TestClaimsLanded_SkillNowExistingFails(t *testing.T) {
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"already-landed-skill": {Tokens: []string{"/ms-already-landed-skill"}},
	}}
	skills := skillUniverse{"ms-already-landed-skill": "plugins/mindspec/skills/ms-already-landed-skill/SKILL.md"}
	findings := checkClaimsLanded(reg, sharedCmdRoot(t), skills)
	requireFindingContains(t, findings, "claim `already-landed-skill`", "the referent has landed")
}

// TestClaimsLanded_VerbNowResolvingFails is the R1 analogue: a
// synthetic registry claiming a verb that DOES resolve against the
// real cmdRoot (using a real, live verb rather than inventing one, so
// the fixture can't itself go stale) must fail.
func TestClaimsLanded_VerbNowResolvingFails(t *testing.T) {
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"already-landed": {Tokens: []string{"mindspec doctor"}},
	}}
	findings := checkClaimsLanded(reg, sharedCmdRoot(t), sharedSkills(t))
	requireFindingContains(t, findings, "claim `already-landed`", "the referent has landed")
}

// TestClaimsLanded_BehavioralOnlyClaimNotForcedByExistence is the
// RED-on-inject regression fixture for final-gate finding L4-FINAL-3:
// a PLACEHOLDER Cobra command that merely resolves (reproducing
// loop-status's real token, `mindspec loop status`, but with no
// behavioral guarantee it actually reports panels/rounds/budget/halt
// state) must NOT force marker removal, and neither must a STUB
// SKILL.md that merely exists (fix-cycle's real token, `/ms-fix-cycle`,
// with no guarantee the skill runs the lane end-to-end). Both claim
// ids' becomes_true_when demands behavior existence-resolution cannot
// observe (see claimLandedRequiresBehavior), so checkClaimsLanded must
// report zero findings even though `doctor` (standing in for a
// scaffolded `loop status`) resolves and the synthetic skill exists.
func TestClaimsLanded_BehavioralOnlyClaimNotForcedByExistence(t *testing.T) {
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		// "mindspec doctor" stands in for a scaffolded-but-not-yet-behavioral
		// `loop status` placeholder: a REAL, resolving verb, used here only
		// to prove existence resolving is not what's stopping the finding —
		// claimLandedRequiresBehavior's exclusion is.
		"loop-status": {Tokens: []string{"mindspec doctor"}},
		"fix-cycle":   {Tokens: []string{"/ms-fix-cycle"}},
	}}
	skills := skillUniverse{"ms-fix-cycle": "plugins/mindspec/skills/ms-fix-cycle/SKILL.md"}
	findings := checkClaimsLanded(reg, sharedCmdRoot(t), skills)
	if len(findings) != 0 {
		t.Fatalf("expected zero findings for behavioral-only claims even though their tokens resolve/exist, got: %v", findings)
	}
}

// TestClaimsLanded_ConfigKeyTokenNeverFlagged pins the documented
// scope limit: a bare config-key-shaped token (no leading "mindspec "
// or "/") is never landed-checkable this way, however it resolves —
// it asserts a BEHAVIOR change the binary makes, not an existence fact
// R1/R2's ground truth can observe.
func TestClaimsLanded_ConfigKeyTokenNeverFlagged(t *testing.T) {
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"loop-governance": {Tokens: []string{"gate_authority", "handoff_log", "max_beads_per_wake"}},
	}}
	findings := checkClaimsLanded(reg, sharedCmdRoot(t), sharedSkills(t))
	if len(findings) != 0 {
		t.Fatalf("expected zero findings for config-key-shaped tokens (not landed-checkable), got: %v", findings)
	}
}

// --- F2-4: owner / locations / becomes_true_when validation -------------

// checkClaimFieldsValid reads and validates the three previously-dead
// registry fields: owner and becomes_true_when must be non-empty
// (the registry's own header says a claim nobody has committed to
// build, with no stated landing condition, is retracted rather than
// registered), and every locations entry must resolve to one of the
// scanned docs AND that doc must actually carry a claim-`id` marker —
// a locations entry pointing at a nonexistent or unmarked file names a
// site nobody can verify.
func checkClaimFieldsValid(docs []*docFile, reg *claimsRegistryDoc) []truthFinding {
	var out []truthFinding

	markersByPath := make(map[string]map[string]bool, len(docs))
	for _, df := range docs {
		set := markersByPath[df.Path]
		if set == nil {
			set = map[string]bool{}
			markersByPath[df.Path] = set
		}
		for _, m := range df.Markers {
			set[m.ID] = true
		}
	}

	ids := make([]string, 0, len(reg.Claims))
	for id := range reg.Claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		claim := reg.Claims[id]
		text := "claim `" + id + "`"

		if strings.TrimSpace(claim.Owner) == "" {
			out = append(out, truthFinding{File: "project-docs/claims-registry.yaml", Text: text, Why: "owner is empty -- a claim nobody has committed to build should be retracted from the docs, not registered"})
		}
		if strings.TrimSpace(claim.BecomesTrueWhen) == "" {
			out = append(out, truthFinding{File: "project-docs/claims-registry.yaml", Text: text, Why: "becomes_true_when is empty -- a claim with no stated landing condition can't be checked for staleness"})
		}
		if len(claim.Locations) == 0 {
			out = append(out, truthFinding{File: "project-docs/claims-registry.yaml", Text: text, Why: "locations is empty -- the claim names no site a reader can verify"})
			continue
		}
		for _, loc := range claim.Locations {
			set, ok := markersByPath[loc]
			if !ok {
				out = append(out, truthFinding{File: "project-docs/claims-registry.yaml", Text: text, Why: fmt.Sprintf("locations entry %q does not resolve to a scanned file", loc)})
				continue
			}
			if !set[id] {
				out = append(out, truthFinding{File: "project-docs/claims-registry.yaml", Text: text, Why: fmt.Sprintf("locations entry %q carries no claim-`%s` marker", loc, id)})
			}
		}
	}
	return out
}

// TestClaimFieldsValid_RealRegistryPasses is the positive sanity check:
// every field of every real, currently-registered claim is well-formed.
func TestClaimFieldsValid_RealRegistryPasses(t *testing.T) {
	repoRoot := realRepoRoot(t)
	relPaths, err := listScopedDocs(repoRoot)
	if err != nil {
		t.Fatalf("listScopedDocs: %v", err)
	}
	var docs []*docFile
	for _, rel := range relPaths {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		df, err := parseDocFile(repoRoot, rel)
		if err != nil {
			t.Fatalf("parseDocFile %s: %v", rel, err)
		}
		docs = append(docs, df)
	}
	findings := checkClaimFieldsValid(docs, sharedRegistry(t))
	if len(findings) != 0 {
		var b []string
		for _, f := range findings {
			b = append(b, f.String())
		}
		t.Fatalf("expected zero field-validity findings for the real registry, got:\n  %s", joinLines(b))
	}
}

// TestClaimFieldsValid_DeletedOwnerBecomesTrueWhenAndBadLocationAllFail
// is the RED-on-inject regression fixture for F2-4, reproducing the
// exact exploit named in the bead brief: "deleting fix-cycle's owner
// AND becomes_true_when, and pointing locations at a nonexistent
// narnia.md, leaves the lint GREEN". All three must now fail.
func TestClaimFieldsValid_DeletedOwnerBecomesTrueWhenAndBadLocationAllFail(t *testing.T) {
	docs := []*docFile{fixtureDoc(t, "guide.md", "Nothing here mentions the claim.\n")}
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"fix-cycle": {
			Tokens:    []string{"/ms-fix-cycle"},
			Owner:     "",
			Locations: []string{"narnia.md"},
			// BecomesTrueWhen deliberately left as its zero value.
		},
	}}
	findings := checkClaimFieldsValid(docs, reg)
	requireFindingContains(t, findings, "claim `fix-cycle`", "owner is empty")
	requireFindingContains(t, findings, "claim `fix-cycle`", "becomes_true_when is empty")
	requireFindingContains(t, findings, "claim `fix-cycle`", "does not resolve to a scanned file")
}

// TestClaimFieldsValid_LocationMissingMarkerFails proves the second
// half of the locations check: a location that DOES exist among the
// scanned docs, but carries no marker for this claim's id, must still
// fail — pointing at a real file is not enough if that file never
// actually names the claim.
func TestClaimFieldsValid_LocationMissingMarkerFails(t *testing.T) {
	docs := []*docFile{fixtureDoc(t, "guide.md", "This file exists but never mentions any claim marker.\n")}
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"fix-cycle": {
			Tokens:          []string{"/ms-fix-cycle"},
			Owner:           "someone",
			Locations:       []string{"guide.md"},
			BecomesTrueWhen: "the skill file exists",
		},
	}}
	findings := checkClaimFieldsValid(docs, reg)
	requireFindingContains(t, findings, "claim `fix-cycle`", "carries no claim-`fix-cycle` marker")
}

// TestClaimFieldsValid_WellFormedEntryPasses is the positive twin: an
// entry with a non-empty owner and becomes_true_when, and a locations
// entry that both exists and carries the matching marker, produces no
// findings.
func TestClaimFieldsValid_WellFormedEntryPasses(t *testing.T) {
	df := fixtureDoc(t, "guide.md", "Some `mindspec loop status` *(planned — claim `loop-status`, roadmap Core 4)* text.\n")
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"loop-status": {
			Tokens:          []string{"mindspec loop status"},
			Owner:           "someone",
			Locations:       []string{"guide.md"},
			BecomesTrueWhen: "the verb exists",
		},
	}}
	findings := checkClaimFieldsValid([]*docFile{df}, reg)
	if len(findings) != 0 {
		t.Fatalf("expected zero findings for a well-formed entry, got: %v", findings)
	}
}
