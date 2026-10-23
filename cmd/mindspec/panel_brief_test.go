package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/rules"
)

// Each fixture commits only inside t.TempDir. No git operation mutates the
// implementation checkout, and no fixture uses a model CLI or network.
func newBriefFixture(t *testing.T, spec, cfg string) (string, string) {
	t.Helper()
	root := t.TempDir()
	subjectCLIGit(t, root, "init", "-q", "-b", "main")
	subjectCLIWrite(t, root, ".mindspec/specs/901-brief/spec.md", spec)
	subjectCLIWrite(t, root, "pkg/required.go", "package pkg\n")
	if cfg != "" {
		subjectCLIWrite(t, root, ".mindspec/config.yaml", cfg)
	}
	subjectCLIGit(t, root, "add", "-A")
	subjectCLIGit(t, root, "commit", "-qm", "reviewed fixture")
	sha := subjectCLIGit(t, root, "rev-parse", "HEAD")
	subjectCLIGit(t, root, "branch", "bead/mindspec-brief.1")
	subjectCLIGit(t, root, "branch", "spec/901-brief")
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	return root, sha
}

func runBriefCreate(t *testing.T, gate string) (string, error) {
	t.Helper()
	readAtRef := fileAtRefForPanelBriefFn
	resetPanelCreateFlags(t)
	fileAtRefForPanelBriefFn = readAtRef
	target := "HEAD"
	if panel.IsLifecycleGate(gate) {
		target = "spec/901-brief"
	}
	args := []string{"create", "p", "--gate", gate}
	if gate != "adhoc" {
		args = append(args, "--spec", "901-brief")
	}
	if gate == "bead" {
		target = "bead/mindspec-brief.1"
	}
	return runPanelVerbCmd(append(args, "--target", target)...)
}

func readCreatedBrief(t *testing.T, root, gate string) string {
	t.Helper()
	dir := filepath.Join(root, ".mindspec/specs/901-brief/reviews/p")
	if gate == "adhoc" {
		dir = filepath.Join(root, ".mindspec/reviews/p")
	}
	data, err := os.ReadFile(filepath.Join(dir, "BRIEF.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func briefRegion(t *testing.T, brief, name string) string {
	t.Helper()
	_, text, ok := strings.Cut(brief, "<!-- mindspec:"+name+" -->\n")
	if !ok {
		t.Fatalf("missing %s region in:\n%s", name, brief)
	}
	text, _, ok = strings.Cut(text, "<!-- /mindspec:"+name+" -->")
	if !ok {
		t.Fatalf("missing closing %s marker", name)
	}
	return text
}

func TestBrief_QuotedACs(t *testing.T) {
	for _, tc := range []struct{ name, spec, want string }{
		{"verbatim", "# Spec\n\n## Acceptance Criteria\n\n- Cite ADR-0045 and R9.3.\n- Read .mindspec/domains/x.md.\n- Create pkg/new.go.\n- Delete pkg/deleted.go.\n\n## Other\nnot quoted\n", "## Acceptance Criteria\n\n- Cite ADR-0045 and R9.3.\n- Read .mindspec/domains/x.md.\n- Create pkg/new.go.\n- Delete pkg/deleted.go.\n\n"},
		{"absent", "# Spec\n\n## Goal\nreview me\n", "no acceptance criteria section found"},
		{"printable", "## Acceptance Criteria\n- [ ] AC-1: say \"hi\" — ok\n", "## Acceptance Criteria\n- [ ] AC-1: say \"hi\" — ok\n"},
		{"control", "## Acceptance Criteria\n- Escape \x1b[31m \u202e \u200b \xff\n", "## Acceptance Criteria\n- Escape \\x1b[31m \\u202e \\u200b \\xff\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := newBriefFixture(t, tc.spec, "")
			// The working copy disagrees. The quote must still use the SHA.
			subjectCLIWrite(t, root, ".mindspec/specs/901-brief/spec.md", "## Acceptance Criteria\nwrong working copy\n")
			if out, err := runBriefCreate(t, "spec_approve"); err != nil {
				t.Fatalf("create: %v\n%s", err, out)
			}
			brief := readCreatedBrief(t, root, "spec_approve")
			quote := briefRegion(t, brief, "spec-acs")
			if !strings.Contains(quote, "```text\n"+tc.want) || strings.Contains(quote, "wrong working copy") || strings.ContainsRune(quote, '\x1b') {
				t.Fatalf("wrong reviewed quotation:\n%s", quote)
			}
			outside := strings.Replace(brief, quote, "", 1)
			if strings.Contains(outside, ".mindspec/domains/") || strings.Contains(outside, "ADR-0045") || strings.Contains(outside, "R9.3") {
				t.Fatalf("internal references leaked outside quotation:\n%s", outside)
			}
		})
	}
}

func TestBrief_Regions(t *testing.T) {
	for _, gate := range config.PanelGateKeys {
		t.Run(gate, func(t *testing.T) {
			root, _ := newBriefFixture(t, "## Acceptance Criteria\n- correct\n", "")
			if out, err := runBriefCreate(t, gate); err != nil {
				t.Fatalf("create: %v\n%s", err, out)
			}
			brief := readCreatedBrief(t, root, gate)
			for _, region := range []string{"acceptance", "spec-acs", "conduct", "verdict-contract"} {
				briefRegion(t, brief, region)
			}
			conduct := briefRegion(t, brief, "conduct")
			all, err := rules.Load()
			if err != nil {
				t.Fatal(err)
			}
			selected, err := rules.Filter("reviewer", gate)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]bool{}
			for _, rule := range selected {
				want[rule.ID] = true
				line := rule.ID + ": " + rule.Prohibition + " — " + rule.Replacement
				if !strings.Contains(conduct, line) {
					t.Errorf("missing conduct line %q", line)
				}
			}
			for _, rule := range all {
				if strings.Contains(conduct, rule.ID+":") != want[rule.ID] {
					t.Errorf("incorrect conduct membership: %s", rule.ID)
				}
			}
			if strings.Contains(conduct, "review.ci-parity:") != (gate == "final_review") {
				t.Fatal("CI parity must be final-review only")
			}
			contract := briefRegion(t, brief, "verdict-contract")
			for _, field := range []string{"schema", "slot", "round", "reviewed_sha", "status", "verdict", "model", "rationale", "concrete_changes_required", "commands_run", "void_reason", "hard_block", "findings", "inputs_read", "mindspec panel schema", "mindspec panel submit"} {
				if !strings.Contains(contract, field) {
					t.Errorf("missing verdict field or verb: %s", field)
				}
			}
			if strings.Contains(brief, "beside this BRIEF") || strings.Contains(brief, "confidence") {
				t.Fatal("obsolete direct-write contract remains")
			}
			if gate == "adhoc" && !strings.Contains(briefRegion(t, brief, "spec-acs"), "none (adhoc panel)") {
				t.Fatal("adhoc panel must have no spec quotation")
			}
		})
	}
}

func TestBrief_AcceptanceRegion(t *testing.T) {
	questions := map[string]string{
		"spec_approve": "Can a planner write a plan from this spec without inventing a requirement?",
		"plan_approve": "Does every bead trace to a requirement, and can each bead be built and tested alone?",
		"bead":         "Does the change meet the bead's acceptance criteria with passing tests, and break nothing else?",
		"final_review": "Does the branch meet every acceptance criterion, and does the project's CI command pass?",
		"adhoc":        "Is the change correct, and does it do what its description says?",
	}
	for gate, question := range questions {
		t.Run(gate, func(t *testing.T) {
			root, _ := newBriefFixture(t, "# Spec\n", "")
			if out, err := runBriefCreate(t, gate); err != nil {
				t.Fatalf("create: %v\n%s", err, out)
			}
			acceptance := briefRegion(t, readCreatedBrief(t, root, gate), "acceptance")
			if !strings.Contains(acceptance, question) {
				t.Fatalf("missing gate question %q:\n%s", question, acceptance)
			}
			cfg, err := config.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range cfg.GateAcceptance(gate).NotBlocking {
				if !strings.Contains(acceptance, "- "+item+"\n") {
					t.Errorf("missing not-blocking item %q", item)
				}
			}
		})
	}
	t.Run("configured", func(t *testing.T) {
		root, _ := newBriefFixture(t, "# Spec\n", "panel:\n  gates:\n    spec_approve:\n      acceptance:\n        question: Custom question?\n        not_blocking: [custom exclusion]\n")
		if out, err := runBriefCreate(t, "spec_approve"); err != nil {
			t.Fatalf("create: %v\n%s", err, out)
		}
		region := briefRegion(t, readCreatedBrief(t, root, "spec_approve"), "acceptance")
		if !strings.Contains(region, "Custom question?") || !strings.Contains(region, "- custom exclusion\n") {
			t.Fatalf("configured acceptance ignored: %s", region)
		}
	})
}

func TestBrief_PathCheck(t *testing.T) {
	for _, tc := range []struct{ name, question, want string }{
		{"present", "Check `pkg/required.go`.", ""},
		{"absent", "Check `pkg/missing.go`.", "absent at the reviewed commit: pkg/missing.go"},
		{"root-extension", "Check `missing.sql`.", "absent at the reviewed commit: missing.sql"},
		{"unmatched-backtick", "Check `literal and ``missing.sql``.", "absent at the reviewed commit: missing.sql"},
		{"dotted-identifier", "Check `config.Load`.", ""},
		{"prefixed-version", "Check `v1.2`.", ""},
		{"numeric-version", "Check `1.5`.", ""},
		{"config-extension", "Check `config.yaml`.", "absent at the reviewed commit: config.yaml"},
		{"doc-extension", "Check `plan.md`.", "documentation path"},
		{"go-extension", "Check `main.go`.", "absent at the reviewed commit: main.go"},
		{"slash-directory", "Check `internal/panel`.", "absent at the reviewed commit: internal/panel"},
		{"slash-file", "Check `a/b.go`.", "absent at the reviewed commit: a/b.go"},
		{"non-paths", "Check and/or CI/CD.", ""},
		{"bare-file", "Check missing.sql.", ""},
		{"command", "Use `mindspec panel/schema`.", ""},
		{"documentation", "Read `.mindspec/domains/x.md`.", "documentation path"},
		{"internal", "Follow ADR-0045.", "internal reference"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := newBriefFixture(t, "## Acceptance Criteria\n- Create pkg/not-yet.go and read ADR-0045.\n", "panel:\n  gates:\n    spec_approve:\n      acceptance:\n        question: "+tc.question+"\n")
			// Presence in the current working tree must not satisfy the check.
			subjectCLIWrite(t, root, "pkg/missing.go", "package pkg\n")
			before := briefFileSnapshot(t, root)
			out, err := runBriefCreate(t, "spec_approve")
			if tc.want == "" {
				if err != nil {
					t.Fatalf("create: %v\n%s", err, out)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "recovery:") {
					t.Fatalf("want refusal %q, got %v\n%s", tc.want, err, out)
				}
				if !reflect.DeepEqual(before, briefFileSnapshot(t, root)) {
					t.Fatal("refused create wrote files")
				}
			}
		})
	}
	// The checker must propagate executor failures, not treat them as absence.
	err := panel.ValidateBriefRegions(panel.BriefRegions{Question: "Check `pkg/required.go`."}, func(string) ([]byte, bool, error) {
		return nil, false, os.ErrPermission
	})
	if err == nil || !strings.Contains(err.Error(), "cannot check review guidance path") {
		t.Fatalf("lost path check failure: %v", err)
	}
	for _, regions := range []panel.BriefRegions{
		{NotBlocking: []string{"Check `missing.sql`."}},
		{Conduct: []panel.BriefRule{{ID: "check", Prohibition: "Read `missing.sql`."}}},
	} {
		if err := panel.ValidateBriefRegions(regions, func(string) ([]byte, bool, error) {
			return nil, false, nil
		}); err == nil || !strings.Contains(err.Error(), "absent at the reviewed commit: missing.sql") {
			t.Fatalf("path outside the question was not checked: %v", err)
		}
	}
}

func briefFileSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestBrief_QuotedFenceSection(t *testing.T) {
	spec := "# Spec\n## Acceptance Criteria\n- example\n```text\n## Not a section\n```\n- keep\n## Next\ndrop\n"
	want := "## Acceptance Criteria\n- example\n```text\n## Not a section\n```\n- keep\n"
	if got := panel.AcceptanceCriteriaSection(spec); got != want {
		t.Fatalf("section changed: got %q want %q", got, want)
	}
}
