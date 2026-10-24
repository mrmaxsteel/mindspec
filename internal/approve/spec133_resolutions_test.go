package approve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/executor"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/phase"
)

func TestS133Resolutions_ApprovalObligations(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, state := range []string{"live", "abandoned", "deleted ledger", "replaced", "ledger only", "outside section", "id prefix", "fenced ids", "comment ids", "all named"} {
			t.Run(policy+"/"+state, func(t *testing.T) {
				f := s133Document(t, policy, "spec_approve", "unresolved")
				id := f.deferFinding(false)
				if _, err := f.approve("spec_approve"); err != nil {
					t.Fatal(err)
				}
				// The epic and ledger must both contribute: the second id exists only
				// on the epic, and the first is removed from that list on ledger cases.
				f.meta["plan_deferral_ids"] = []string{"plan-epic-only"}
				if state == "deleted ledger" {
					f.meta["plan_deferral_ids"] = []string{id, "plan-epic-only"}
				}
				ledger := filepath.Join(f.dir, "reviews/spec_approve/dispositions.jsonl")
				if state != "deleted ledger" {
					preflightGit(t, f.root, "add", "-f", ledger)
					preflightGit(t, f.root, "commit", "-qm", "track spec ledger")
				}
				switch state {
				case "abandoned":
					dir := filepath.Dir(ledger)
					res, err := panel.Tally(dir)
					if err != nil {
						t.Fatal(err)
					}
					paneltest.Seed(t, dir, panel.AbandonRecord{Record: panel.RecordAbandon, Spec: s133DocID, Panel: "spec_approve", Reason: "review stopped", By: "operator", CreatedAt: paneltest.CreatedAt})
					res.Panel.Abandoned = true
					res.Panel.AbandonReason = "review stopped"
					if err := panel.WriteSnapshot(dir, *res.Panel); err != nil {
						t.Fatal(err)
					}
				case "deleted ledger":
					preflightGit(t, f.root, "add", "-f", ledger)
					preflightGit(t, f.root, "commit", "-qm", "track before deletion")
					if err := os.Rename(ledger, filepath.Join(t.TempDir(), "deleted-ledger")); err != nil {
						t.Fatal(err)
					}
				case "ledger only":
					if err := os.Rename(filepath.Join(filepath.Dir(ledger), panel.FileName), filepath.Join(t.TempDir(), "old-snapshot")); err != nil {
						t.Fatal(err)
					}
				case "replaced":
					res, err := panel.Tally(filepath.Dir(ledger))
					if err != nil {
						t.Fatal(err)
					}
					p := res.Panel
					if err := panel.Create(filepath.Dir(ledger), panel.CreateInput{Spec: s133DocID, Target: p.Target, Gate: "spec_approve", Round: 2, ReviewedHeadSHA: p.ReviewedHeadSHA, SubjectDigest: p.SubjectDigest, Roster: p.Roster}); err != nil {
						t.Fatal(err)
					}
				}
				content := strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1)
				content += "\n## Resolutions\n\n"
				if state == "all named" {
					content += "Resolve `" + id + "` and `plan-epic-only`.\n"
				} else if state == "id prefix" {
					content += id + "-extra plan-epic-only-extra\n"
				} else if state == "fenced ids" {
					content += "````md\n```\n" + id + " plan-epic-only\n```\n````\n"
				} else if state == "comment ids" {
					content += "<!--\n" + id + " plan-epic-only\n-->\n"
				} else {
					content += "None.\n"
				}
				if state == "outside section" {
					content += "\n## Appendix\n\n" + id + " plan-epic-only\n"
				}
				f.writePlan(content)
				f.commit("ready plan")
				f.seed("plan_approve", "pass")
				f.exec.commits = 0
				f.mutations = nil
				if state == "all named" {
					if _, err := f.approve("plan_approve"); err != nil {
						t.Fatal(err)
					}
					if f.creates != 1 {
						t.Fatal("resolution-complete plan made no bead")
					}
					return
				}
				before := f.read("plan.md")
				head := preflightGit(t, f.root, "rev-parse", "HEAD")
				_, err := f.approve("plan_approve")
				if err == nil || !strings.Contains(err.Error(), id) || !strings.Contains(err.Error(), "plan-epic-only") {
					t.Fatalf("missing obligations not named: %v", err)
				}
				if before != f.read("plan.md") || head != preflightGit(t, f.root, "rev-parse", "HEAD") || len(f.mutations) != 0 || f.closes != 0 || f.creates != 0 || f.exec.commits != 0 {
					t.Fatalf("resolution refusal mutated: %v", f.mutations)
				}
			})
		}
	}
}

func TestS133Resolutions_SectionBoundary(t *testing.T) {
	for _, text := range []string{"## Appendix\nplan-one", "## Resolutions\nNone\n## Appendix\nplan-one", "```md\n## Resolutions\nplan-one\n```", "## Resolutions\nplan-one-extra"} {
		if resolutionNamesID(planResolutionsSection(text), "plan-one") {
			t.Fatalf("accepted outside or partial id: %s", text)
		}
	}
	if !resolutionNamesID(planResolutionsSection("## Resolutions\n### Decision\nResolve `plan-one`.\n"), "plan-one") {
		t.Fatal("nested resolution did not name id")
	}
}

func TestS133Resolutions_HiddenMarkdown(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"shorter backticks", "````md\n```\n## Resolutions\nplan-one\n```\n````\n"},
		{"different fence character", "~~~md\n```\n## Resolutions\nplan-one\n~~~\n"},
		{"closing fence info", "```md\n```text\n## Resolutions\nplan-one\n```\n"},
		{"indented closing fence", "```md\n    ```\n## Resolutions\nplan-one\n```\n"},
		{"nonbreaking space closer", "```md\n```\u00a0\n## Resolutions\nplan-one\n```\n"},
		{"comment heading", "<!--\n## Resolutions\nplan-one\n-->\n"},
		{"single line comment heading", "<!-- ## Resolutions plan-one -->\n"},
		{"fenced id", "## Resolutions\n```\nplan-one\n```\n"},
		{"tilde fenced id", "## Resolutions\n~~~\nplan-one\n~~~\n"},
		{"comment id", "## Resolutions\n<!-- plan-one -->\n"},
		{"multiline comment id", "## Resolutions\n<!--\nplan-one\n-->\n"},
		{"comment boundary", "## Resolutions\n<!--\n## Appendix\n-->\nNone.\n"},
		{"comment inside fence", "```\n<!--\n```\n## Appendix\nplan-one\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if resolutionNamesID(planResolutionsSection(tc.text), "plan-one") {
				t.Fatalf("hidden id counted: %s", tc.text)
			}
			for _, control := range []string{"Resolve plan-one.\n", "- Resolve `plan-one`.\n"} {
				text := tc.text + "\n## Resolutions\n" + control
				if !resolutionNamesID(planResolutionsSection(text), "plan-one") {
					t.Fatalf("visible control did not count: %s", text)
				}
			}
		})
	}
	for _, text := range []string{
		"````\n```\n````` \t\n## Resolutions\nplan-one\n",
		"   ~~~md\n~~~\r\n## Resolutions\nplan-one\n",
		"## Resolutions\n<!--\n## Appendix\n-->\nplan-one\n",
		"## Resolutions\n<!-- hidden -->Resolve plan-one.\n",
		"## Resolutions\n```bad`info\nplan-one\n",
	} {
		if !resolutionNamesID(planResolutionsSection(text), "plan-one") {
			t.Fatalf("visible id did not count: %s", text)
		}
	}
}

var resolutionBlockCases = []struct{ name, tail string }{
	{"empty bullet heading", "\n-\n  ## Resolutions\n  plan-one\n"},
	{"empty ordered heading", "\n1.\n   ## Resolutions\n   plan-one\n"},
	{"list fence blank", "\n## Plan\n- item\n  ```\n  code\n\n  ## Resolutions\n  R1 resolved\n  ```\n"},
	{"inline tag heading boundary", "\n## Resolutions\n\nR2 resolved\n\n## Bead 1: handle `<id>` input\n\nR1 resolved.\n"},
	{"list fence", "\n## Resolutions\n\n- item\n    ```\n    plan-one\n    ```\n"},
	{"quote fence", "\n## Resolutions\n\n> ```\n> plan-one\n> ```\n"},
	{"indented heading", "\n    ## Resolutions\n    plan-one\n"},
	{"indented text", "\n## Resolutions\n\n    plan-one\n"},
	{"list heading", "\n- item\n  ## Resolutions\n  plan-one\n"},
	{"list heading on marker", "\n- ## Resolutions\n  plan-one\n"},
	{"quote heading", "\n> ## Resolutions\n> plan-one\n"},
	{"list fence in quote", "\n## Resolutions\n\n> - item\n>     ```\n>     plan-one\n>     ```\n"},
	{"quote fence in list", "\n## Resolutions\n\n- item\n  > ~~~\n  > plan-one\n  > ~~~\n"},
	{"nested list fence", "\n## Resolutions\n\n- item\n  - child\n    ~~~~\n    plan-one\n    ~~~~\n"},
	{"ordered list fence", "\n## Resolutions\n\n10. item\n    ```\n    plan-one\n    ```\n"},
	{"ordered marker in paragraph", "\n## Resolutions\n\nParagraph.\n2. prose\n\n    ```\n    plan-one\n    ```\n"},
	{"ordered marker after continuation", "\n## Resolutions\n\nParagraph.\n    continuation\n2. prose\n\n    ```\n    plan-one\n    ```\n"},
	{"nonbreaking space in paragraph", "\n## Resolutions\n\nParagraph.\n\u00a0\n2. prose\n\n    plan-one\n"},
	{"ordered marker after lazy quote", "\n## Resolutions\n\n> Paragraph.\n2. prose\n\n    plan-one\n"},
	{"ordered marker after lazy list", "\n## Resolutions\n\n- Paragraph.\n2. prose\n\n    plan-one\n"},
	{"fence on list marker", "\n## Resolutions\n\n- ```\n  plan-one\n  ```\n"},
	{"code with list marker", "\n## Resolutions\n\n    - plan-one\n"},
	{"code with quote marker", "\n## Resolutions\n\n    > plan-one\n"},
	{"code in list", "\n## Resolutions\n\n- item\n\n      plan-one\n"},
	{"code after thematic break", "\n## Resolutions\n\n- - -\n\n    plan-one\n"},
	{"tab code", "\n## Resolutions\n\n\tplan-one\n"},
	{"tab heading", "\n\t## Resolutions\n\tplan-one\n"},
	{"carriage return code", "\r## Resolutions\r\r    plan-one\r"},
	{"comment list marker", "\n## Resolutions\n<!--\n- item\n-->\n    plan-one\n"},
	{"list comment before fence", "\n## Resolutions\n\n- <!-- hidden -->\n    ```\n  plan-one\n    ```\n"},
	{"split comment id", "\n## Resolutions\nA-<!-- -->1\n"},
	{"tab section boundary", "\n## Resolutions\nNone.\n##\tAppendix\nplan-one\n"},
	{"empty section boundary", "\n## Resolutions\nNone.\n##\nplan-one\n"},
	{"closing hashes boundary", "\n## Resolutions\nNone.\n## Appendix ##\nplan-one\n"},
	{"setext boundary", "\n## Resolutions\n\nAppendix\n--------\nplan-one\n"},
	{"multiline setext boundary", "\n## Resolutions\n\nplan-one\nAppendix\n--------\n"},
	{"link destination", "\n## Resolutions\n\n[decision](plan-one)\n"},
	{"multiline link destination", "\n## Resolutions\n\n[decision](\nplan-one\n)\n"},
	{"script heading after blank", "\n<script>\n\n## Resolutions\nplan-one\n</script>\n"},
	{"preformatted text after blank", "\n## Resolutions\n\n<pre>\n\nplan-one\n</pre>\n"},
}

var resolutionBlockControls = []string{
	"Resolve plan-one.\n",
	"- Resolve `plan-one`.\n",
	"- Decision\n  - Resolve plan-one.\n",
}

func TestS133Resolutions_BlockStructure(t *testing.T) {
	for _, tc := range resolutionBlockCases {
		t.Run(tc.name, func(t *testing.T) {
			id := "plan-one"
			if tc.name == "split comment id" {
				id = "A-1"
			} else if tc.name == "list fence blank" || tc.name == "inline tag heading boundary" {
				id = "R1"
			}
			if section := planResolutionsSection("# Plan\n\nBody.\n" + tc.tail); resolutionNamesID(section, id) {
				t.Fatalf("hidden id counted in %q", section)
			}
			for _, control := range resolutionBlockControls {
				text := tc.tail + "\n## Resolutions\n\n" + strings.ReplaceAll(control, "plan-one", id)
				if !resolutionNamesID(planResolutionsSection(text), id) {
					t.Fatalf("visible control did not count: %s", text)
				}
			}
		})
	}
	for _, text := range []string{
		"## Resolutions\n~~old~~\nResolve plan-one.\n",
		"   ## Resolutions ###\nResolve plan-one.\n",
		"##\tResolutions\r\nResolve plan-one.\r\n",
	} {
		if !resolutionNamesID(planResolutionsSection(text), "plan-one") {
			t.Fatalf("visible id did not count: %s", text)
		}
	}
}

func TestS133Resolutions_BlockStructureApprovePlan(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for i, tc := range resolutionBlockCases {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				f := s133Document(t, policy, "plan_approve", "absent")
				id := "plan-one"
				if tc.name == "split comment id" {
					id = "A-1"
				} else if tc.name == "list fence blank" || tc.name == "inline tag heading boundary" {
					id = "R1"
				}
				f.meta["plan_deferral_ids"] = []string{id}
				content := strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1)
				tail := tc.tail
				if tc.name == "inline tag heading boundary" {
					content = strings.Replace(content, "## Bead 1: First thing", "## Resolutions\nR2 resolved\n\n## Bead 1: handle `<id>` input\n\nR1 resolved\n", 1)
					tail = ""
				}
				f.writePlan(content + tail)
				f.commit("block structure refusal")
				f.seed("plan_approve", "pass")
				f.exec.commits = 0
				f.mutations = nil
				before := f.read("plan.md")
				head := preflightGit(t, f.root, "rev-parse", "HEAD")
				_, err := f.approve("plan_approve")
				if err == nil || !strings.Contains(err.Error(), "missing deferred finding ids: "+id) {
					t.Errorf("hidden obligation did not refuse: %v (creates=%d)", err, f.creates)
				}
				if f.creates != 0 || f.closes != 0 || f.exec.commits != 0 || len(f.mutations) != 0 || before != f.read("plan.md") || head != preflightGit(t, f.root, "rev-parse", "HEAD") {
					t.Errorf("refusal mutated lifecycle: creates=%d closes=%d commits=%d mutations=%v", f.creates, f.closes, f.exec.commits, f.mutations)
				}
				if t.Failed() {
					return
				}
				control := strings.ReplaceAll(resolutionBlockControls[i%len(resolutionBlockControls)], "plan-one", id)
				f = s133Document(t, policy, "plan_approve", "absent")
				f.meta["plan_deferral_ids"] = []string{id}
				f.writePlan(content + tail + "\n## Resolutions\n\n" + control)
				f.commit("visible block structure control")
				f.seed("plan_approve", "pass")
				if _, err := f.approve("plan_approve"); err != nil {
					t.Fatalf("visible control refused: %v", err)
				}
				if f.creates != 1 {
					t.Fatalf("visible control created %d beads, want 1", f.creates)
				}
			})
		}
	}
}

func TestS133Resolutions_RefusalBeforeWrites(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "plan_approve", "abandoned")
			f.children = []string{"old-one", "old-two"}
			f.meta["plan_deferral_ids"] = []string{"plan-missing"}
			delete(f.meta, "mindspec_phase")
			delete(f.meta, phase.PhaseSemanticsV2MarkerKey)
			migrations := 0
			t.Cleanup(phase.SetMergeMetadataForTest(func(_ string, updates map[string]interface{}) error {
				migrations++
				f.mutations = append(f.mutations, "migration")
				for key, value := range updates {
					f.meta[key] = value
				}
				return nil
			}))
			before := f.read("plan.md")
			head := preflightGit(t, f.root, "rev-parse", "HEAD")
			metadata, err := json.Marshal(f.meta)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.approve("plan_approve")
			if err == nil || !strings.Contains(err.Error(), "plan resolutions:") || !strings.Contains(err.Error(), "plan-missing") {
				t.Fatalf("missing resolution did not refuse: %v", err)
			}
			if f.closes != 0 || f.creates != 0 || len(f.mutations) != 0 || migrations != 0 {
				t.Errorf("resolution refusal wrote lifecycle: closes=%d creates=%d migrations=%d mutations=%v", f.closes, f.creates, migrations, f.mutations)
			}
			afterMetadata, err := json.Marshal(f.meta)
			if err != nil || string(metadata) != string(afterMetadata) {
				t.Errorf("resolution refusal changed metadata: %s -> %s (%v)", metadata, afterMetadata, err)
			}
			if _, err := os.Stat(filepath.Join(f.root, phase.PhaseLockRelPath)); !os.IsNotExist(err) {
				t.Errorf("resolution refusal reached migration lock: %v", err)
			}
			if before != f.read("plan.md") || head != preflightGit(t, f.root, "rev-parse", "HEAD") || f.exec.commits != 0 {
				t.Error("resolution refusal wrote the plan or made a commit")
			}
			if t.Failed() {
				return
			}

			// The control proves that each pending write has a live path.
			f.writePlan(before + "\nResolve plan-missing.\n")
			f.commit("resolve finding")
			f.exec.commits = 0
			if _, err := f.approve("plan_approve"); err != nil {
				t.Fatal(err)
			}
			if f.closes != 1 || f.creates != 1 || migrations == 0 || f.meta["panel_abandoned"] != true || f.meta["mindspec_migrated_at"] == nil {
				t.Fatalf("control did not reach pending writes: closes=%d creates=%d migrations=%d metadata=%v", f.closes, f.creates, migrations, f.meta)
			}
		})
	}
}

func TestS133Resolutions_TrackedRoundGate(t *testing.T) {
	f := s133Document(t, "enforce", "spec_approve", "unresolved")
	id := f.deferFinding(false)
	lines, err := panel.ReadLedger(filepath.Join(f.dir, "reviews/spec_approve"))
	if err != nil {
		t.Fatal(err)
	}
	for _, gate := range []string{"spec_approve", "plan_approve"} {
		t.Run(gate, func(t *testing.T) {
			var records [][]byte
			for _, line := range lines {
				var record map[string]interface{}
				if err := json.Unmarshal(line, &record); err != nil {
					t.Fatal(err)
				}
				if record["record"] == panel.RecordRound {
					record["gate"] = gate
				}
				data, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				records = append(records, data)
			}
			ids := map[string]bool{}
			if err := collectPlanDeferralIDs(records, s133DocID, ids); err != nil {
				t.Fatal(err)
			}
			want := map[string]bool{}
			if gate == "spec_approve" {
				want[id] = true
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("wrong obligations for round gate %s: got %v, want %v", gate, ids, want)
			}
		})
	}
}

func TestS133Resolutions_ReApprovalRetainsAcceptedIDs(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "spec_approve", "unresolved")
			id := f.deferFinding(false)
			f.epic = true
			f.meta["plan_deferral_ids"] = []string{"plan-older-deleted-ledger"}
			if _, err := f.approve("spec_approve"); err != nil {
				t.Fatal(err)
			}
			ids, err := recordedPlanDeferralIDs(f.meta)
			if err != nil || len(ids) != 2 || !resolutionNamesID(strings.Join(ids, " "), id) || !resolutionNamesID(strings.Join(ids, " "), "plan-older-deleted-ledger") {
				t.Fatalf("accepted obligation lost: %v %v", ids, err)
			}
		})
	}
}

func TestS133Resolutions_TrackedCommitUsesSpecCheckout(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		t.Run(policy, func(t *testing.T) {
			f := s133Document(t, policy, "spec_approve", "unresolved")
			id := f.deferFinding(false)
			ledger := filepath.Join(f.dir, "reviews/spec_approve/dispositions.jsonl")
			preflightGit(t, f.root, "add", "-f", ledger)
			preflightGit(t, f.root, "commit", "-qm", "tracked ledger on spec only")
			primary := filepath.Join(t.TempDir(), "primary")
			preflightGit(t, f.root, "worktree", "add", "--detach", primary, "HEAD~1")
			if err := os.Rename(filepath.Join(filepath.Dir(ledger), panel.FileName), filepath.Join(t.TempDir(), "snapshot")); err != nil {
				t.Fatal(err)
			}
			exec := executor.NewMindspecExecutor(primary)
			err := checkPlanResolutions(f.root, s133DocID, "## Resolutions\nNone.\n", nil, exec)
			if err == nil || !strings.Contains(err.Error(), id) {
				t.Fatalf("read primary HEAD instead of spec commit: %v", err)
			}
			ids, err := specPlanDeferralIDs(f.root, f.root, s133DocID, exec)
			if err != nil || len(ids) != 1 || ids[0] != id {
				t.Fatalf("ledger-only obligation lost at spec approval: %v %v", ids, err)
			}
		})
	}
}

func TestS133Resolutions_QuoteFenceBlankControl(t *testing.T) {
	text := "> ```\n> code\n\n## Resolutions\nR1 resolved\n"
	if !resolutionNamesID(planResolutionsSection(text), "R1") {
		t.Fatal("blank line did not end quote fence")
	}
}

var resolutionInlineCorpus = []struct {
	name, body string
	ids        []string
	want       bool
}{
	{"boundary autolink label", "See [a <https://x/> b](plan-one) done.\n", []string{"plan-one"}, false},
	{"boundary autolink bracket", "See [a <https://x/]> b](plan-one) done.\n", []string{"plan-one"}, false},
	{"boundary html label", "See [a <b> c](plan-one).\n", []string{"plan-one"}, false},
	{"boundary code label", "See [a `c` d](plan-one).\n", []string{"plan-one"}, false},
	{"boundary unmatched backticks", "See ``[x](`plan-one) done.\n", []string{"plan-one"}, false},
	{"boundary literal backticks control", "See ``[x](`u) plan-one done.\n", []string{"plan-one"}, true},
	{"boundary lowercase declaration", "See <!plan-one> done.\n", []string{"plan-one"}, false},
	{"boundary lowercase doctype", "See <!doctype plan-one> done.\n", []string{"plan-one"}, false},
	{"grammar unicode full fold", "See ![plan-one][Straße].\n\n[STRASSE]: /u\n", []string{"plan-one"}, false},
	{"grammar unicode simple fold", "See ![plan-one][Σ].\n\n[ς]: /u\n", []string{"plan-one"}, false},
	{"grammar reference nonbreaking space", "See ![plan-one][a b].\n\n[a\u00a0b]: /u\n", []string{"plan-one"}, true},
	{"grammar nested link", "See [a [b](u)](plan-one).\n", []string{"plan-one"}, true},
	{"grammar nested image", "See ![a ![b](u[) c](plan-one).\n", []string{"plan-one"}, false},
	{"grammar nested image in link", "See [a ![plan-one](u)](v).\n", []string{"plan-one"}, false},
	{"grammar autolink image label", "See ![a <https://x/]> b](plan-one).\n", []string{"plan-one"}, false},
	{"grammar probe complete destination", "See [decision](plan-one).\n", []string{"plan-one"}, false},
	{"grammar reference whitespace fold", "See [decision][the plan-one].\n\n[THE\tPLAN-ONE]: /u\n", []string{"plan-one"}, false},
	{"grammar empty image target", "See ![plan-one]().\n", []string{"plan-one"}, false},
	{"grammar empty angle image target", "See ![plan-one](<>).\n", []string{"plan-one"}, false},
	{"grammar invalid title parentheses", "See [x](u (a(plan-one))).\n", []string{"plan-one"}, true},
	{"grammar escaped title parentheses", "See [x](u (a\\(plan-one\\))).\n", []string{"plan-one"}, false},
	{"grammar escaped angle delimiter", "See [x](<a\\>plan-one>).\n", []string{"plan-one"}, false},
	{"grammar angle unescaped less than", "See [x](<a<plan-one>).\n", []string{"plan-one"}, false},
	{"grammar bare destination control", "See [x](u\u0001plan-one).\n", []string{"plan-one"}, true},
	{"grammar bare destination delete", "See [x](u\u007fplan-one).\n", []string{"plan-one"}, true},
	{"grammar definition title separator", "[r]: <u>\"plan-one\"\n", []string{"plan-one"}, true},
	{"grammar definition title extra", "[r]: u \"a\" plan-one\n", []string{"plan-one"}, true},
	{"grammar definition next line invalid title", "[r]: u\n\"a\" plan-one\n", []string{"plan-one"}, true},
	{"grammar image undefined collapsed", "See ![plan-one][].\n", []string{"plan-one"}, true},
	{"grammar image undefined shortcut", "See ![plan-one].\n", []string{"plan-one"}, true},
	{"grammar image code label", "See ![`plan-one`](u).\n", []string{"plan-one"}, false},
	{"grammar link code label", "See [`plan-one`](u).\n", []string{"plan-one"}, true},
	{"grammar code closing bracket", "See [a `]` b](plan-one).\n", []string{"plan-one"}, false},
	{"grammar image alt", "See ![plan-one](x.png).\n", []string{"plan-one"}, false},
	{"grammar image reference alt", "See ![plan-one][r].\n\n[r]: x.png\n", []string{"plan-one"}, false},
	{"grammar image invalid target", "See ![a](x plan-one).\n", []string{"plan-one"}, true},
	{"grammar invalid target words", "See [x](u plan-one).\n", []string{"plan-one"}, true},
	{"grammar invalid target title suffix", "See [x](u \"a\" plan-one).\n", []string{"plan-one"}, true},
	{"grammar undefined reference", "See [decision][plan-one].\n", []string{"plan-one"}, true},
	{"grammar defined reference", "See [decision][plan-one].\n\n[plan-one]: /u\n", []string{"plan-one"}, false},
	{"grammar invalid decision target", "See [decision](x plan-one).\n", []string{"plan-one"}, true},
	{"grammar invalid decision angle", "See [decision](<x y> plan-one).\n", []string{"plan-one"}, true},
	{"grammar destination internal less than", "See [x](a<plan-one) done.\n", []string{"plan-one"}, false},
	{"grammar title internal less than", "See [x](a (plan-one<)) done.\n", []string{"plan-one"}, false},
	{"grammar definition internal less than", "[b]: a<plan-one\n", []string{"plan-one"}, false},
	{"grammar list definition internal less than", "- [b]: x<plan-one\n", []string{"plan-one"}, false},
	{"grammar invalid destination spaces", "See [x](a b plan-one) done.\n", []string{"plan-one"}, true},
	{"grammar invalid destination newline", "See [x](c\nplan-one) done.\n", []string{"plan-one"}, true},
	{"grammar invalid title trailing words", "See [x](c \"t\" plan-one) done.\n", []string{"plan-one"}, true},
	// These invalid links contain a complete raw HTML tag. Its attributes stay hidden.
	{"grammar invalid internal angle spaces", "See [x](a<b plan-one>) done.\n", []string{"plan-one"}, false},
	{"grammar invalid angle newline", "[a](<b\nplan-one>)\n", []string{"plan-one"}, false},
	{"grammar invalid bare newline", "[a](plan-one\nmore)\n", []string{"plan-one"}, true},
	{"grammar invalid angle missing separator", "[a](<u>plan-one)\n", []string{"plan-one"}, true},
	{"grammar autolink label bracket", "[a <https://x/]> b](plan-one)\n", []string{"plan-one"}, false},
	{"grammar reference normalization", "See [decision][ PLAN-ONE ].\n\n[plan-one]: /u\n", []string{"plan-one"}, false},
	{"grammar collapsed image", "See ![plan-one][].\n\n[PLAN-ONE]: /u\n", []string{"plan-one"}, false},
	{"grammar shortcut image", "See ![plan-one].\n\n[PLAN-ONE]: /u\n", []string{"plan-one"}, false},
	{"grammar undefined image reference", "See ![decision][plan-one].\n", []string{"plan-one"}, true},
	{"grammar code definition is not registered", "`[r]: /u`\n\nSee ![plan-one][r].\n", []string{"plan-one"}, true},
	{"grammar paragraph definition is not registered", "text\n[r]: /u\n\nSee ![plan-one][r].\n", []string{"plan-one"}, true},
	{"grammar invalid definition is not registered", "[r]: /u junk\n\nSee ![plan-one][r].\n", []string{"plan-one"}, true},
	{"hidden closing tag", "See </plan-one> done.\n", []string{"plan-one"}, false},
	{"hidden custom tag", "See <plan-one> done.\n", []string{"plan-one"}, false},
	{"single quoted html attribute", "See <a href='plan-one'>x</a>.\n", []string{"plan-one"}, false},
	{"html label bracket destination", "See [x <a title=\"[\">y</a>](plan-one).\n", []string{"plan-one"}, false},
	{"html label closing bracket destination", "See [x <a title=\"]\">y</a>](plan-one).\n", []string{"plan-one"}, false},
	{"code label bracket destination", "See [x `[` y](plan-one).\n", []string{"plan-one"}, false},
	{"escaped bracket literal", "See \\[ plan-one.\n", []string{"plan-one"}, true},
	{"unmatched code run", "See ``broken ` plan-one.\n", []string{"plan-one"}, true},
	{"reference destination eof", "[b]: plan-one", []string{"plan-one"}, false},
	{"reference title eof", "[b]: x \"plan-one\"", []string{"plan-one"}, false},
	{"reference angle eof", "[b]: <plan-one>", []string{"plan-one"}, false},
	{"reference escaped destination", "[b]: a\\(plan-one\\)", []string{"plan-one"}, false},
	{"reference balanced destination", "[b]: a(plan-one)", []string{"plan-one"}, false},
	{"invalid reference title suffix", "[b]: c \"title\" plan-one\n", []string{"plan-one"}, true},
	{"invalid reference destination suffix", "[b]: c plan-one\n", []string{"plan-one"}, true},
	{"half open range", "- Half-open range [0, n) fixed: plan-one.\n", []string{"plan-one"}, true},
	{"unmatched bracket note", "Note [ x plan-one\n", []string{"plan-one"}, true},
	{"unclosed code and bracket", "- `[ unclosed plan-one\n", []string{"plan-one"}, true},
	{"unmatched bracket item", "- a [ b plan-one\n", []string{"plan-one"}, true},
	{"unmatched outer bracket", "- [a [b](c) plan-one\n", []string{"plan-one"}, true},
	{"unfinished target continuation", "See [b](c\nplan-one fixed.\n", []string{"plan-one"}, true},
	{"unfinished target same line", "See [b](c plan-one fixed.\n", []string{"plan-one"}, true},
	{"unfinished angle target", "See [b](<c plan-one\n", []string{"plan-one"}, true},
	{"unfinished quoted target", "See [b](c \"plan-one\n", []string{"plan-one"}, true},
	{"unfinished target title", "See [b](c (plan-one\n", []string{"plan-one"}, true},
	{"unfinished reference suffix", "See [a][b plan-one\n", []string{"plan-one"}, true},
	{"unfinished reference angle", "[b]: <c plan-one\n", []string{"plan-one"}, true},
	{"unfinished reference title", "[b]: c \"plan-one\n", []string{"plan-one"}, true},
	{"unfinished reference next title", "[b]: c\n\"plan-one\n", []string{"plan-one"}, true},
	{"reference without destination", "[plan-one]:\n", []string{"plan-one"}, true},
	{"unfinished reference parentheses", "[b]: (c plan-one\n", []string{"plan-one"}, true},
	{"literal less than", "a < b plan-one\n", []string{"plan-one"}, true},
	{"visible html content", "<b>plan-one</b> fixed\n", []string{"plan-one"}, true},
	{"after complete link same line", "See [b](c). plan-one fixed.\n", []string{"plan-one"}, true},
	{"hidden html href", "See <a href=\"plan-one\">decision</a>.\n", []string{"plan-one"}, false},
	{"hidden html title", "See <span title=\"plan-one\">decision</span>.\n", []string{"plan-one"}, false},
	{"hidden html short href", "See <a href=\"plan-one\">x</a>\n", []string{"plan-one"}, false},
	{"hidden html short title", "See <span title=\"plan-one\">x</span>\n", []string{"plan-one"}, false},
	{"hidden inline comment", "See <!-- plan-one --> done.\n", []string{"plan-one"}, false},
	{"hidden processing instruction", "See <?plan-one ?> done.\n", []string{"plan-one"}, false},
	{"hidden declaration", "See <!THING plan-one> done.\n", []string{"plan-one"}, false},
	{"hidden cdata", "See <![CDATA[plan-one]]> done.\n", []string{"plan-one"}, false},
	{"html quoted greater than", "See <a title=\"> plan-one\">x</a> done.\n", []string{"plan-one"}, false},
	{"html unquoted attribute", "See <a href=plan-one>x</a> done.\n", []string{"plan-one"}, false},
	{"html multiline attribute", "See <a\nhref=\"plan-one\">x</a>.\n", []string{"plan-one"}, false},
	{"after html tag", "See <a href=\"hidden\">x</a> plan-one.\n", []string{"plan-one"}, true},
	{"invalid html tag", "See <a href=\"unfinished plan-one\n", []string{"plan-one"}, true},
	{"unfinished processing instruction", "See <?broken plan-one\n", []string{"plan-one"}, true},
	{"unfinished declaration", "See <!THING plan-one\n", []string{"plan-one"}, true},
	{"unfinished cdata", "See <![CDATA[plan-one\n", []string{"plan-one"}, true},
	{"url autolink", "See <https://example.test/plan-one>.\n", []string{"plan-one"}, true},
	{"email autolink", "See <plan-one@example.test>.\n", []string{"plan-one"}, true},
	{"html in code", "See `<a href=\"plan-one\">` done.\n", []string{"plan-one"}, true},
	{"after lazy paragraph", "> Context.\nlazy text\n\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"checkbox", "- [x] Resolve plan-one and plan-epic-only.\n", []string{"plan-one", "plan-epic-only"}, true},
	{"after link", "- See [the ADR](x.md).\n- plan-one: resolved.\n", []string{"plan-one"}, true},
	{"bracket code same line", "- plan-one: returns `[]string` now.\n- plan-two: fixed.\n", []string{"plan-one"}, true},
	{"bracket code later line", "- plan-one: returns `[]string` now.\n- plan-two: fixed.\n", []string{"plan-two"}, true},
	{"after apostrophe destination", "- See [decision](path'part). plan-one: fixed.\n", []string{"plan-one"}, true},
	{"angle destination", "- See [decision](<path).plan-one>).\n", []string{"plan-one"}, false},
	{"parenthesis title", "- See [decision](x.md (plan-one)).\n", []string{"plan-one"}, false},
	{"link destination", "- see [decision](plan-one)\n", []string{"plan-one"}, false},
	{"link title", "- see [decision](x.md \"plan-one\")\n", []string{"plan-one"}, false},
	{"plain", "- plan-one: fixed.\n", []string{"plan-one"}, true},
	{"nested link label", "- [[plan-one]: fixed](x.md)\n", []string{"plan-one"}, true},
	{"link label suffix", "- [plan-one](x.md)-extra\n", []string{"plan-one"}, false},
	{"link label", "- [plan-one](x.md): fixed.\n", []string{"plan-one"}, true},
	{"after title", "- see [decision](x.md \"hidden\")\n- plan-one: fixed.\n", []string{"plan-one"}, true},
	{"reference definition", "[decision]: x.md \"plan-one\"\n", []string{"plan-one"}, false},
	{"after reference definition", "[decision]: x.md\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"reference continuation title", "[decision]: x.md\n\"plan-one\"\n", []string{"plan-one"}, false},
	{"after reference title", "[decision]: x.md\n\"hidden title\"\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"reference chained destination", "[one]: x.md\n[two]: plan-one\n", []string{"plan-one"}, false},
	{"reference chained label", "[one]: x.md\n[plan-one]: y.md\n", []string{"plan-one"}, false},
	{"reference multiline title", "[decision]: x.md \"title\nplan-one\nend\"\n", []string{"plan-one"}, false},
	{"after reference multiline title", "[decision]: x.md \"title\nhidden\nend\"\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"reference suffix", "[decision][plan-one]\n", []string{"plan-one"}, true},
	{"multiline title", "[decision](x.md\n\"plan-one\")\n", []string{"plan-one"}, false},
	{"incomplete target next item", "- [decision](unfinished\n- plan-one: fixed.\n", []string{"plan-one"}, true},
	{"incomplete bracket next line", "[unfinished\nplan-one: fixed.\n", []string{"plan-one"}, true},
	{"incomplete target next paragraph", "[decision](unfinished\n\nplan-one: fixed.\n", []string{"plan-one"}, true},
}

func TestS133Resolutions_InlineCorpus(t *testing.T) {
	for _, tc := range resolutionInlineCorpus {
		t.Run(tc.name, func(t *testing.T) {
			section := planResolutionsSection("# T\n\n## Resolutions\n\n" + tc.body + "\n## Next\n")
			for _, id := range tc.ids {
				if got := resolutionNamesID(section, id); got != tc.want {
					t.Errorf("id %s counts=%v, want %v; section=%q", id, got, tc.want, section)
				}
			}
		})
	}
}

func TestS133Resolutions_InlineApprovePlan(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, tc := range resolutionInlineCorpus {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				content := strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1)
				checkResolutionApprovePlan(t, policy, content+"\n## Resolutions\n\n"+tc.body, tc.ids, tc.want)
			})
		}
	}
}

// These cases isolate inline rules from the block scanner.
func TestS133Resolutions_InlineSyntax(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"code trim one space", "` plan-one `suffix", "plan-onesuffix"},
		{"code keep all spaces", "`   `plan-one", "   plan-one"},
		{"code trim only one", "`  plan-one  `suffix", " plan-one suffix"},
		{"code normalize newline", "`\nplan-one\n`suffix", "plan-onesuffix"},
		{"comment", "a <!-- plan-one --> b", "a  b"},
		{"unfinished comment", "a <!-- plan-one", "a <!-- plan-one"},
		{"invalid comment", "a <!--> plan-one -->", "a <!--> plan-one -->"},
		{"processing instruction", "a <?plan-one ?> b", "a  b"},
		{"unfinished instruction", "a <?plan-one", "a <?plan-one"},
		{"declaration", "a <!THING plan-one> b", "a  b"},
		{"unfinished declaration", "a <!THING plan-one", "a <!THING plan-one"},
		{"cdata", "a <![CDATA[plan-one]]> b", "a  b"},
		{"unfinished cdata", "a <![CDATA[plan-one", "a <![CDATA[plan-one"},
		{"empty comment", "a <!----> b", "a  b"},
		{"closing tag", "a </plan-one> b", "a  b"},
		{"visible content", "<b>plan-one</b>", "plan-one"},
		{"quote in tag", "<a title=\"> plan-one\">x</a>", "x"},
		{"code tag", "`<a title=\"plan-one\">`", "<a title=\"plan-one\">"},
		{"invalid tag", "a < b plan-one", "a < b plan-one"},
		{"url", "<https://example.test/plan-one>", "https://example.test/plan-one"},
		{"email", "<plan-one@example.test>", "plan-one@example.test"},
		{"trailing escape", "plan-one\\", "plan-one\\"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolutionVisibleText(tc.body); got != tc.want {
				t.Fatalf("visible text=%q, want %q", got, tc.want)
			}
		})
	}
}

// These cases check delimiter matching and inline precedence.
func TestS133Resolutions_EmphasisSyntax(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"hidden prefix dual star", "<b>*\"one\"*.", "\"one\"."},
		{"hidden prefix dual underscore", "<b>_\"one\"_.", "\"one\"."},
		{"hidden prefix dual triple", "<b>***\"one\"***.", "\"one\"."},
		{"strong", "plan-**one**: resolved.", "plan-one: resolved."},
		{"italic", "plan-*one*: resolved.", "plan-one: resolved."},
		{"underscore punctuation", "plan-_one_: resolved.", "plan-one: resolved."},
		{"suffix", "**plan-one**suffix", "plan-onesuffix"},
		{"intraword star", "foo*bar*baz", "foobarbaz"},
		{"intraword underscore", "foo_bar_baz", "foo_bar_baz"},
		{"nonflanking star", "a*\"foo\"*", "a*\"foo\"*"},
		{"nonflanking underscore", "a_\"foo\"_", "a_\"foo\"_"},
		{"unmatched", "a * plan-one", "a * plan-one"},
		{"space after opener", "* foo*", "* foo*"},
		{"space before closer", "*foo *", "*foo *"},
		{"nonbreaking whitespace", "* foo *", "* foo *"},
		{"unicode punctuation", "«_one_»", "«one»"},
		{"unicode symbol", "£_one_£", "£one£"},
		{"unicode letter", "é_one_é", "é_one_é"},
		{"unicode astral punctuation", "😀_one_😀", "😀one😀"},
		{"unmatched underscore", "_one*", "_one*"},
		{"triple", "***one***", "one"},
		{"long runs", "****one****", "one"},
		{"partly unmatched", "**one*", "*one"},
		{"nested mixed", "*_one_*", "one"},
		{"nested strong", "**one *two* three**", "one two three"},
		{"overlap", "*one _two* three_", "one _two three_"},
		{"shorter opener", "**one **two three**", "**one two three"},
		{"rule of three blocks", "a**one*two", "a**one*two"},
		{"rule of three allows", "a***one***two", "aonetwo"},
		{"rule of three remainder", "***one** two*", "one two"},
		{"rule of three nested", "*one**two**three*", "onetwothree"},
		{"multiple closers", "*one**two***", "onetwo"},
		{"underscore nested", "__one _two_ three__", "one two three"},
		{"underscore before punctuation", "_(one)_.", "(one)."},
		{"escaped opener", "\\*one*", "*one*"},
		{"escaped closer", "*one\\*", "*one*"},
		{"escaped inner", "*one\\**", "one*"},
		{"escaped run boundary", "\\**one*", "*one"},
		{"code precedence", "*a `*` b*", "a * b"},
		{"code contents literal", "`**one**`", "**one**"},
		{"autolink precedence", "*<https://x/*>*", "https://x/*"},
		{"html attribute precedence", "*a <b title=\"*\">c</b> d*", "a c d"},
		{"link label", "[plan-**one**](u)", "plan-one"},
		{"link label bounds before", "*[one*](u)", "*one*"},
		{"link label bounds after", "[one*](u)two*", "one*two*"},
		{"emphasis around link", "*[one](u)*", "one"},
		{"failed link allows pair", "*[one*](u bad)", "[one](u bad)"},
		{"image label cannot close", "*![one*](u)", "*"},
		{"image label cannot open", "![*one](u)*", "*"},
		{"reference label emphasis", "[r]: /u\n[plan-**one**][r]", "plan-one"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolutionVisibleText(tc.body); got != tc.want {
				t.Fatalf("visible text=%q, want %q", got, tc.want)
			}
		})
	}
}

// Target validity is separate from other markup in a failed link.
func TestS133Resolutions_LinkTargetGrammar(t *testing.T) {
	cases := []struct {
		name, target  string
		inline, valid bool
	}{
		{"internal angle", "a<plan-one)", true, true},
		{"parenthesis title", "a (plan-one<))", true, true},
		{"invalid bare words", "a b plan-one)", true, false},
		{"invalid angle words", "a<b plan-one>)", true, false},
		{"invalid angle newline", "<b\nplan-one>)", true, false},
		{"angle missing separator", "<u>plan-one)", true, false},
		{"invalid title words", "c \"t\" plan-one)", true, false},
		{"destination newline words", "plan-one\nmore)", true, false},
		{"definition internal angle", "a<plan-one", false, true},
		{"definition title", "a (plan-one<)", false, true},
		{"definition invalid words", "a b plan-one", false, false},
		{"definition invalid title words", "c \"t\" plan-one", false, false},
		{"blank line before closer", "u\n\n)", true, false},
		{"blank line in title", "u \"a\n\nplan-one\")", true, false},
		{"newline before closer", "u\n)", true, true},
		{"escaped newline in angle", "<a\\\nplan-one>)", true, false},
		{"unclosed angle", "<plan-one)", true, false},
		{"unclosed quote", "u \"plan-one)", true, false},
		{"unbalanced parentheses", "a(plan-one)", true, false},
		{"escaped parentheses", "a\\(plan-one\\))", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			end := resolutionLinkTarget(tc.target, 0, tc.inline)
			if (end >= 0) != tc.valid {
				t.Fatalf("target end=%d, valid=%v", end, tc.valid)
			}
			if tc.valid && end != len(tc.target) {
				t.Fatalf("target end=%d, want %d", end, len(tc.target))
			}
		})
	}
}

func checkResolutionApprovePlan(t *testing.T, policy, content string, ids []string, want bool) {
	t.Helper()
	f := s133Document(t, policy, "plan_approve", "absent")
	f.meta["plan_deferral_ids"] = ids
	f.writePlan(content)
	f.commit("inline resolutions")
	f.seed("plan_approve", "pass")
	f.exec.commits = 0
	f.mutations = nil
	before := f.read("plan.md")
	head := preflightGit(t, f.root, "rev-parse", "HEAD")
	metadata, err := json.Marshal(f.meta)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.approve("plan_approve")
	if want {
		if err != nil || f.creates != 1 {
			t.Fatalf("visible ids refused: %v; creates=%d", err, f.creates)
		}
	} else {
		if err == nil || !strings.Contains(err.Error(), "hidden text") || f.creates != 0 {
			t.Fatalf("hidden ids did not refuse clearly: %v; creates=%d", err, f.creates)
		}
		afterMetadata, marshalErr := json.Marshal(f.meta)
		if marshalErr != nil || string(metadata) != string(afterMetadata) || f.closes != 0 || f.exec.commits != 0 || len(f.mutations) != 0 || before != f.read("plan.md") || head != preflightGit(t, f.root, "rev-parse", "HEAD") {
			t.Errorf("inline refusal changed lifecycle state: mutations=%v, metadata error=%v", f.mutations, marshalErr)
		}
		for _, id := range ids {
			if !strings.Contains(err.Error(), id) {
				t.Errorf("refusal omits %s: %v", id, err)
			}
		}
	}
}

var resolutionDocumentCorpus = []struct {
	name, doc string
	want      bool
}{
	{"continuation indented definition title", "## Resolutions\n\n[r]: /u\n    \"a\nplan-one\nend\"\n", false},
	{"continuation tab definition title", "## Resolutions\n\n[r]: /u\n\t\"a\nplan-one\nend\"\n", false},
	{"continuation definition title reference", "## Resolutions\n\nSee ![plan-one][r].\n\n[r]: /u\n    \"a\nb\nend\"\n", false},
	{"continuation invalid indented definition title", "## Resolutions\n\n[r]: /u\n    \"plan-one\n", false},
	{"continuation after definition code", "## Resolutions\n\n[r]: /u\n    hidden\nplan-one\n", true},
	{"reviewer exact emphasis visible", "## Resolutions\n\nplan-**one**: resolved.\n", true},
	{"reviewer exact emphasis longer id", "## Resolutions\n\n**plan-one**suffix: resolved.\n", false},
	{"reviewer exact lazy quote", "## Resolutions\n\n> first\nplan-one done\n", true},
	{"reviewer exact lazy list", "## Resolutions\n\n- first\nplan-one done\n", true},
	{"sweep code trim prefix", "## Resolutions\n\nplan-` one `: resolved.\n", true},
	{"sweep code trim suffix", "## Resolutions\n\n` plan-one `suffix\n", false},
	{"sweep autolink suffix", "## Resolutions\n\n<https://x/plan-one>suffix\n", false},
	{"sweep autolink prefix", "## Resolutions\n\nprefix<plan-one@example.test>\n", false},
	{"continuation indented fence is paragraph text", "## Resolutions\n\n- item\n  > para\n      ```\n      plan-one\n      ```\n", true},
	{"continuation nested list quote fence", "## Resolutions\n\n- item\n  > para\n    ```\n    plan-one\n    ```\n", false},
	{"continuation list quote ends fence", "## Resolutions\n\n- > para\n\n  paragraph\n    ```\n    plan-one\n    ```\n", false},
	{"continuation loose list fence", "## Resolutions\n\n- item\n\n  paragraph\n    ```\n    plan-one\n    ```\n", false},
	{"continuation uncertain whole block", "## Resolutions\n\n> plan-one\nlazy\n> > tail\n", false},
	{"continuation uncertain no definition", "## Resolutions\n\nSee ![plan-one][r].\n\n> [r]: /u \"a\nlazy\n> > tail\"\n", true},
	{"continuation code after definition", "## Resolutions\n\n[r]: /u\n    plan-one\n", false},
	{"continuation visible old block control", "# Plan\n\nBody.\n\n## Resolutions\n> Context.\nlazy text\nplan-one\n", true},
	{"continuation indented title", "# P\n\n## Resolutions\n\nSee [x](u \"a\n    b\nplan-one\") done.\n", false},
	{"continuation tab title", "# P\n\n## Resolutions\n\nSee [x](u \"a\n\tb\nplan-one\") done.\n", false},
	{"continuation indented destination", "# P\n\n## Resolutions\n\nSee [x](\n    u\n\"plan-one\") done.\n", false},
	{"continuation indented alt", "# P\n\n## Resolutions\n\nSee ![a\n    b\nplan-one](u) done.\n", false},
	{"continuation blank code control", "# P\n\n## Resolutions\n\nintro\n\n    plan-one\n", false},
	{"continuation lazy quote destination", "# P\n\n## Resolutions\n\n> See [x](plan-one\n)\n", false},
	{"continuation lazy list destination", "# P\n\n## Resolutions\n\n- See [x](plan-one\n)\n", false},
	{"continuation lazy quote title", "# P\n\n## Resolutions\n\n> See [x](u \"plan-one\n\")\n", false},
	{"continuation lazy list title", "# P\n\n## Resolutions\n\n- See [x](u \"plan-one\n\")\n", false},
	{"continuation lazy ordered destination", "# P\n\n## Resolutions\n\n1. See [x](plan-one\n)\n", false},
	{"continuation lazy nested destination", "# P\n\n## Resolutions\n\n> > See [x](plan-one\n)\n", false},
	{"continuation lazy quote html", "# P\n\n## Resolutions\n\n> See <a title=\"plan-one\n\">x</a>\n", false},
	{"continuation lazy list html", "# P\n\n## Resolutions\n\n- See <a title=\"plan-one\n\">x</a>\n", false},
	{"continuation lazy quote image", "# P\n\n## Resolutions\n\n> See ![x](plan-one\n)\n", false},
	{"continuation lazy quote image alt", "# P\n\n## Resolutions\n\n> See ![plan-one\n](u)\n", false},
	{"continuation lazy list image alt", "# P\n\n## Resolutions\n\n- See ![plan-one\n](u)\n", false},
	{"continuation lazy reference", "# P\n\n## Resolutions\n\n> See [x][plan-one\n]\n\n[plan-one]: /u\n", false},
	{"continuation lazy quote visible", "# P\n\n## Resolutions\n\n> first\nplan-one done", true},
	{"continuation lazy list visible", "# P\n\n## Resolutions\n\n- first\nplan-one done", true},
	{"continuation rejoined quote", "# P\n\n## Resolutions\n\n> See [x](u \"a\nb\n> plan-one\")\n", false},
	{"continuation rejoined list", "# P\n\n## Resolutions\n\n- See [x](u \"a\nb\n  plan-one\")\n", false},
	{"continuation indented html", "# P\n\n## Resolutions\n\nSee <a title=\"a\n    b\nplan-one\">x</a>\n", false},
	{"continuation indented reference", "# P\n\n## Resolutions\n\nSee ![plan-one][a b c].\n\n[a\n    b\nc]: /u\n", false},
	{"emphasis inside id", "# P\n\n## Resolutions\n\nplan-**one**: resolved.\n", true},
	{"emphasis glued suffix", "# P\n\n## Resolutions\n\n**plan-one**suffix: resolved.\n", false},
	{"emphasis whole id", "# P\n\n## Resolutions\n\n**plan-one**: resolved.\n", true},
	{"emphasis underscore inside id", "# P\n\n## Resolutions\n\nplan-_one_: resolved.\n", true},
	{"emphasis italic inside id", "# P\n\n## Resolutions\n\nplan-*one*: resolved.\n", true},
	{"emphasis italic glued suffix", "# P\n\n## Resolutions\n\n*plan-one*suffix: resolved.\n", false},
	{"emphasis literal stars", "# P\n\n## Resolutions\n\n* plan-one: resolved.\n", true},
	{"emphasis literal unmatched", "# P\n\n## Resolutions\n\na * plan-one", true},
	{"emphasis literal underscore", "# P\n\n## Resolutions\n\nsnake_case plan-one", true},
	{"emphasis code control", "# P\n\n## Resolutions\n\n`**`plan-one", true},
	{"emphasis visible link", "# P\n\n## Resolutions\n\n[plan-**one**](u): resolved.\n", true},
	{"emphasis link suffix", "# P\n\n## Resolutions\n\n[**plan-one**](u)suffix: resolved.\n", false},
	{"continuation spaces thematic not boundary", "# P\n\n## Resolutions\n\nSee ![plan-one][r].\n\nintro\n    ***\n[r]: /u\n", true},
	{"continuation spaces heading not boundary", "# P\n\n## Resolutions\n\nSee ![plan-one][r].\n\nintro\n    ### H\n[r]: /u\n", true},
	{"continuation tab thematic not boundary", "# P\n\n## Resolutions\n\nSee ![plan-one][r].\n\nintro\n\t***\n[r]: /u\n", true},
	{"continuation tab heading not boundary", "# P\n\n## Resolutions\n\nSee ![plan-one][r].\n\nintro\n\t# H\n[r]: /u\n", true},
	{"continuation five spaces thematic not boundary", "# P\n\n## Resolutions\n\nSee ![plan-one][r].\n\nintro\n     - - -\n[r]: /u\n", true},
	{"requested exact quote destination eof", "## Resolutions\n\n> See [x](plan-one\n)", false},
	{"requested exact indented title", "## Resolutions\n\nSee [x](u \"a\n    b\nplan-one\") done.\n", false},
	{"requested exact emphasis inside", "## Resolutions\n\nplan-**one**: resolved.", true},
	{"requested exact emphasis suffix", "## Resolutions\n\n**plan-one**suffix: resolved.", false},
	{"requested exact lazy quote visible", "## Resolutions\n\n> first\nplan-one done", true},
	{"requested exact lazy list visible", "## Resolutions\n\n- first\nplan-one done", true},

	{"boundary autolink label", "# P\n\n## Resolutions\n\nSee [a <https://x/> b](plan-one) done.\n", false},
	{"boundary autolink bracket", "# P\n\n## Resolutions\n\nSee [a <https://x/]> b](plan-one) done.\n", false},
	{"boundary unmatched backticks", "# P\n\n## Resolutions\n\nSee ``[x](`plan-one) done.\n", false},
	{"boundary lowercase declaration", "# P\n\n## Resolutions\n\nSee <!plan-one> done.\n", false},
	{"boundary lowercase doctype", "# P\n\n## Resolutions\n\nSee <!doctype plan-one> done.\n", false},
	{"boundary definition before section", "# P\n\n[r]: /u\n\n## Resolutions\n\nSee ![plan-one][r].\n", false},
	{"boundary definition after section", "# P\n\n## Resolutions\n\nSee ![plan-one][r].\n\n## Other\n\n[r]: /u\n", false},
	{"boundary definition after heading", "# P\n\n## Resolutions\n\nSee ![plan-one][r].\n### H\n[r]: /u\n", false},
	{"boundary definition after thematic break", "# P\n\n## Resolutions\n\nSee ![plan-one][r].\n***\n[r]: /u\n", false},
	{"boundary definition in quote", "# P\n\n## Resolutions\n\nSee ![plan-one][r].\n\nintro\n> [r]: /u\n", false},
	{"boundary paragraph definition control", "# P\n\n## Resolutions\n\nSee [plan-one][r].\nintro\n[r]: /u\n", true},
	{"boundary quote interrupts", "# P\n\n## Resolutions\n\nSee [x](u\n> \"plan-one\")\n", true},
	{"boundary plain control", "# P\n\n## Resolutions\n\n- plan-one: fixed.\n", true},
	{"boundary suffix definition before", "# P\n\n[plan-one]: /u\n\n## Resolutions\n\nSee [decision][plan-one].\n", false},
	{"boundary suffix definition after", "# P\n\n## Resolutions\n\nSee [decision][plan-one].\n\n## Other\n\n[plan-one]: /u\n", false},
	{"boundary quote continuation control", "## Resolutions\n\n> See [x](u\n> \"plan-one\")\n", false},
	{"boundary nested quote interrupts", "## Resolutions\n\n> See [x](u \"\n>> plan-one\")\n", true},
	{"boundary quote ends", "## Resolutions\n\n> See [x](u \"\n\nplan-one\")\n", true},
	{"boundary list interrupts", "## Resolutions\n\nSee [x](u \"\n- plan-one\")\n", true},
	{"boundary quoted list interrupts", "## Resolutions\n\n> See [x](u \"\n> - plan-one\")\n", true},
	{"boundary heading interrupts", "## Resolutions\n\nSee [x](u \"\n### H\nplan-one\")\n", true},
	{"boundary thematic break interrupts", "## Resolutions\n\nSee [x](u \"\n***\nplan-one\")\n", true},
	{"boundary fence interrupts", "## Resolutions\n\nSee [x](u \"\n```\nhidden\n```\nplan-one\")\n", true},
	{"boundary quote blank definition", "## Resolutions\n\nSee ![plan-one][r].\n\n> intro\n>\n> [r]: /u\n", false},
	{"boundary list definition", "## Resolutions\n\nSee ![plan-one][r].\n\nintro\n- [r]: /u\n", false},
	{"boundary ordered paragraph control", "## Resolutions\n\nintro\n2. text\n[r]: /u\n\nSee ![plan-one][r].\n", true},
	{"boundary fenced definition control", "```\n[r]: /u\n```\n\n## Resolutions\n\nSee ![plan-one][r].\n", true},
	{"boundary definition after heading 1", "# H\n[r]: /u\n\n## Resolutions\n\nSee ![plan-one][r].\n", false},
	{"boundary definition after heading 2", "## H\n[r]: /u\n\n## Resolutions\n\nSee ![plan-one][r].\n", false},
	{"boundary definition after heading 4", "#### H\n[r]: /u\n\n## Resolutions\n\nSee ![plan-one][r].\n", false},
	{"boundary definition after heading 5", "##### H\n[r]: /u\n\n## Resolutions\n\nSee ![plan-one][r].\n", false},
	{"boundary definition after heading 6", "###### H\n[r]: /u\n\n## Resolutions\n\nSee ![plan-one][r].\n", false},
	{"boundary nonbreaking blank control", "## Resolutions\n\nintro\n\u00a0\n[r]: /u\n\nSee ![plan-one][r].\n", true},
	{"boundary nonbreaking thematic control", "## Resolutions\n\nintro\n\u00a0***\n[r]: /u\n\nSee ![plan-one][r].\n", true},
	{"boundary ordered tab title control", "## Resolutions\n\nSee [x](u \"\n2.\tplan-one\")\n", false},
	{"boundary ordered comment title control", "## Resolutions\n\nSee [x](u \"\n2. <!-- hidden -->plan-one\")\n", false},
	{"boundary comment list interrupts", "## Resolutions\n\nSee [x](u \"\n- <!-- hidden -->- plan-one\")\n", true},
	{"boundary comment quote interrupts", "## Resolutions\n\nSee [x](u \"\n> <!-- hidden -->> plan-one\")\n", true},
}

func TestS133Resolutions_DocumentCorpus(t *testing.T) {
	for _, tc := range resolutionDocumentCorpus {
		t.Run(tc.name, func(t *testing.T) {
			section := planResolutionsSection(tc.doc)
			if got := resolutionNamesID(section, "plan-one"); got != tc.want {
				t.Fatalf("counts=%v, want %v; section=%q", got, tc.want, section)
			}
		})
	}
}

func TestS133Resolutions_DocumentApprovePlan(t *testing.T) {
	for _, policy := range []string{"warn", "enforce"} {
		for _, tc := range resolutionDocumentCorpus {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				content := strings.Replace(strings.ReplaceAll(validSingleBeadPlan, "042-test", s133DocID), "version: \"1.0\"", "version: \"1.0\"\nadr_citations: [ADR-0001]", 1)
				checkResolutionApprovePlan(t, policy, content+"\n"+tc.doc, []string{"plan-one"}, tc.want)
			})
		}
	}
}
