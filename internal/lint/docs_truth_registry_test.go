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

func TestClaimsTokensMatchGolden(t *testing.T) {
	reg := sharedRegistry(t)

	seen := make(map[string]bool, len(goldenClaimTokens))
	ids := make([]string, 0, len(reg.Claims))
	for id := range reg.Claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		seen[id] = true
		golden, ok := goldenClaimTokens[id]
		if !ok {
			t.Errorf("claims-registry.yaml has claim `%s` with no entry in goldenClaimTokens -- add one, auditing its exact token set, in this same change", id)
			continue
		}
		if !stringSlicesEqual(reg.Claims[id].Tokens, golden) {
			t.Errorf("claim `%s` tokens changed: registry has %v, golden has %v -- if this edit is audited, update goldenClaimTokens in this SAME change; if you did not make this edit, this is exactly the token-widening smuggle this ratchet exists to catch", id, reg.Claims[id].Tokens, golden)
		}
	}
	for id := range goldenClaimTokens {
		if !seen[id] {
			t.Errorf("golden entry for claim `%s` no longer exists in claims-registry.yaml -- stale golden entry, remove it", id)
		}
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
// regression fixture for A1: simulates exactly the reported exploit
// (appending a fictional token to an existing claim's tokens) against
// a synthetic registry + golden table pair, independent of the real
// files, so the assertion is pinned even if the real registry
// changes shape later.
func TestClaimsTokensMatchGolden_DetectsWidening(t *testing.T) {
	golden := map[string][]string{"loop-status": {"mindspec loop status"}}
	widened := map[string]claimEntry{
		"loop-status": {Tokens: []string{"mindspec loop status", "mindspec telepathy sync --brainwave"}},
	}
	for id, claim := range widened {
		g, ok := golden[id]
		if !ok {
			t.Fatalf("test setup error: no golden entry for %s", id)
		}
		if stringSlicesEqual(claim.Tokens, g) {
			t.Fatal("expected the widened token set to differ from golden, so the ratchet's own comparison is exercised")
		}
	}
}

// --- F2-3 / clause 4: has a claim's referent landed? --------------------

// checkClaimsLanded enforces clause 4 for the checkable subset of
// claims: a token shaped like a `mindspec ...` invocation is resolved
// against cmdRoot (the same real-binary ground truth R1 uses); a
// token shaped like a `/name` skill reference is looked up in skills
// (the same ground truth R2 uses). If either now resolves/exists, the
// claim's referent has landed for real and the entry should have been
// deleted in the same change that landed it.
func checkClaimsLanded(reg *claimsRegistryDoc, cmdRoot *cmdNode, skills skillUniverse) []truthFinding {
	var out []truthFinding

	ids := make([]string, 0, len(reg.Claims))
	for id := range reg.Claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
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
// named in the bead brief: "creating
// plugins/mindspec/skills/ms-fix-cycle/SKILL.md with markers and
// registry untouched leaves the lint GREEN". A synthetic skillUniverse
// standing in for that landed skill, plus the real registry (which
// still has the fix-cycle entry), must now fail.
func TestClaimsLanded_SkillNowExistingFails(t *testing.T) {
	reg := &claimsRegistryDoc{Version: 1, Claims: map[string]claimEntry{
		"fix-cycle": {Tokens: []string{"/ms-fix-cycle"}},
	}}
	skills := skillUniverse{"ms-fix-cycle": "plugins/mindspec/skills/ms-fix-cycle/SKILL.md"}
	findings := checkClaimsLanded(reg, sharedCmdRoot(t), skills)
	requireFindingContains(t, findings, "claim `fix-cycle`", "the referent has landed")
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
