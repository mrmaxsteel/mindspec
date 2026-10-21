package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

func promptFixture(t *testing.T, schema int, cfg string) (string, paneltest.Fixture) {
	t.Helper()
	root := mkPanelTestRoot(t, cfg)
	subjectCLIWrite(t, root, ".mindspec/specs/test/spec.md", "# Spec\n")
	withTestChdir(t, root)
	config.ResetCache()
	t.Cleanup(config.ResetCache)
	p := panel.Panel{Spec: "test", Target: "main", Round: 1, ExpectedReviewers: 2, ReviewedHeadSHA: paneltest.ReviewedSHA, SubjectDigest: "sha256:fixture", Gate: "bead", Telemetry: "required", Roster: []panel.RosterSlot{
		{Slot: "R1", Family: "harness", Model: "m", Lens: "empirical-prober"},
		{Slot: "R2", Family: "harness", Model: "other-model", Lens: "adversarial"},
	}}
	f := paneltest.Fixture{Dir: filepath.Join(root, ".mindspec/specs/test/reviews/p"), Panel: p}
	if err := panel.WriteSnapshot(f.Dir, p); err != nil {
		t.Fatal(err)
	}
	f.Round(t, schema)
	subjectCLIWrite(t, f.Dir, "BRIEF.md", "Review this fixture.\n")
	for _, name := range []string{"slot", "spec"} {
		flag := panelPromptCmd.Flags().Lookup(name)
		old, changed := flag.Value.String(), flag.Changed
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
		t.Cleanup(func() { _ = flag.Value.Set(old); flag.Changed = changed })
	}
	return root, f
}

func promptRun(t *testing.T, root, slot string) string {
	t.Helper()
	before := briefFileSnapshot(t, root)
	out, err := runPanelVerbCmd("prompt", "p", "--slot", slot)
	if err != nil {
		t.Fatalf("prompt: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(before, briefFileSnapshot(t, root)) {
		t.Fatal("panel prompt wrote files")
	}
	return out
}

func TestPanelPrompt_Render(t *testing.T) {
	const processConfig = "panel:\n  runners:\n    - {family: worker, kind: process, argv: [fake-reviewer], prompt: stdin, output: stdout}\n"
	for _, name := range []string{"review-é", "review-é 'quoted' \"path\" $value `literal`"} {
		t.Run("harness-path/"+name, func(t *testing.T) {
			slotDir := filepath.Join(t.TempDir(), name)
			out := panel.RenderSlotPrompt(panel.PromptInput{Kind: "harness", SlotDir: slotDir, Attempt: 1})
			_, argument, ok := strings.Cut(out, "mindspec panel submit --file ")
			if !ok {
				t.Fatalf("missing submission command:\n%s", out)
			}
			argument, _, _ = strings.Cut(argument, "\n")
			// Let a POSIX shell parse the word without running the command.
			parsed, err := exec.Command("sh", "-c", "set -- "+argument+"; printf '%s\\n' \"$#\" \"$1\"").CombinedOutput()
			if err != nil {
				t.Fatalf("parse submission argument: %v\n%s", err, parsed)
			}
			want := "1\n" + filepath.Join(slotDir, "attempt-1", "verdict.json") + "\n"
			if string(parsed) != want {
				t.Fatalf("wrong --file shell argument: got %q, want %q", parsed, want)
			}
		})
	}
	for _, suffix := range []string{"\x1b", "\n", "\x7f", "\u202e", "\xff"} {
		t.Run(fmt.Sprintf("harness-unsafe-path/%x", suffix), func(t *testing.T) {
			out := panel.RenderSlotPrompt(panel.PromptInput{Kind: "harness", SlotDir: filepath.Join(t.TempDir(), "review"+suffix), Attempt: 1})
			if strings.Contains(out, "mindspec panel submit --file ") || !strings.Contains(out, "Cannot render the submission command:") {
				t.Fatalf("unsafe path must refuse the command:\n%s", out)
			}
			if strings.Contains(out, "review"+suffix) {
				t.Fatalf("unsafe path bytes in prompt: %q", out)
			}
		})
	}
	for _, tc := range []struct{ name, brief, want string }{
		{"brief-printable", "check.rule: check this — use that\n", "check.rule: check this — use that\n"},
		{"brief-control", "Escape \x1b[31mtext\n", "Escape \\x1b[31mtext\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, f := promptFixture(t, 2, "")
			subjectCLIWrite(t, f.Dir, "BRIEF.md", tc.brief)
			out := promptRun(t, root, "R1")
			if !strings.HasPrefix(out, tc.want+"\n\n## Slot\n") || strings.ContainsRune(out, '\x1b') {
				t.Fatalf("stored BRIEF changed or contains a control byte:\n%s", out)
			}
		})
	}
	t.Run("harness", func(t *testing.T) {
		root, f := promptFixture(t, 2, "")
		paneltest.Seed(t, f.Dir, f.Launch("dispatched", 1, 1), f.Launch("launch_failed", 1, 1), f.Launch("dispatched", 1, 3))
		out := promptRun(t, root, "R1")
		for _, want := range []string{"Review this fixture.", "Slot: R1", "Family: harness", "Model (the verdict model value): m", "Lens: empirical-prober", "Run the tests and validators. Do the claims hold?", filepath.Join(root, ".mindspec-scratch/test/p/r1/R1"), filepath.Join(root, ".mindspec-scratch/test/p/r1/R1/tree"), "mindspec panel submit --file", "attempt-3/verdict.json", "Write scratch only in the slot directory", "no prior-round items"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing prompt value %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "attempt-1/verdict.json") || strings.Contains(out, "MINDSPEC-VERDICT-BEGIN") {
			t.Fatal("wrong harness output instruction")
		}
		// No dispatch is not an extra refusal and invents no attempt path.
		out = promptRun(t, root, "R2")
		if !strings.Contains(out, "--dispatch") || strings.Contains(out, "attempt-0") {
			t.Fatal("undispatched slot must request dispatch without inventing an attempt")
		}
	})
	t.Run("process", func(t *testing.T) {
		root, f := promptFixture(t, 2, processConfig)
		f.Panel.Roster[0].Family = "worker"
		if err := panel.WriteSnapshot(f.Dir, f.Panel); err != nil {
			t.Fatal(err)
		}
		out := promptRun(t, root, "R1")
		var begin, end string
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "MINDSPEC-VERDICT-BEGIN ") {
				begin = strings.TrimPrefix(line, "MINDSPEC-VERDICT-BEGIN ")
			}
			if strings.HasPrefix(line, "MINDSPEC-VERDICT-END ") {
				end = strings.TrimPrefix(line, "MINDSPEC-VERDICT-END ")
			}
		}
		if begin == "" || begin != end || !strings.Contains(out, "on stdout") || !strings.Contains(out, "Family: worker") {
			t.Fatalf("missing matching process nonce markers:\n%s", out)
		}
		instruction := panel.RenderProcessOutput("stdout", "known-nonce", "unused")
		if !strings.Contains(instruction, "MINDSPEC-VERDICT-BEGIN known-nonce\nMINDSPEC-VERDICT-END known-nonce") {
			t.Fatalf("launcher nonce was not used: %s", instruction)
		}
	})
	t.Run("file", func(t *testing.T) {
		root, f := promptFixture(t, 2, strings.Replace(processConfig, "output: stdout", "output: file", 1))
		f.Panel.Roster[0].Family = "worker"
		if err := panel.WriteSnapshot(f.Dir, f.Panel); err != nil {
			t.Fatal(err)
		}
		subjectCLIWrite(t, filepath.Join(root, ".mindspec-scratch/test/p/r1/R1/attempt-4"), "scratch", "old")
		out := promptRun(t, root, "R1")
		want := filepath.Join(root, ".mindspec-scratch/test/p/r1/R1/attempt-5/verdict.json")
		if !strings.Contains(out, want) || strings.Contains(out, "MINDSPEC-VERDICT-BEGIN") {
			t.Fatalf("wrong file output instruction:\n%s", out)
		}
	})
	t.Run("substitute", func(t *testing.T) {
		root, f := promptFixture(t, 2, processConfig)
		paneltest.Seed(t, f.Dir, panel.SubstitutionRecord{Record: panel.RecordSubstitution, Spec: "test", Panel: "p", Slot: "R1", Round: 1, FromFamily: "harness", ToFamily: "worker", Model: "substitute-model", CreatedAt: paneltest.CreatedAt})
		out := promptRun(t, root, "R1")
		if !strings.Contains(out, "Family: worker") || !strings.Contains(out, "Model (the verdict model value): substitute-model") || !strings.Contains(out, "MINDSPEC-VERDICT-BEGIN") {
			t.Fatalf("substitution was not applied:\n%s", out)
		}
	})
	t.Run("control", func(t *testing.T) {
		root, f := promptFixture(t, 2, "panel:\n  lenses:\n    hostile: \"focus\\e[31m\"\n")
		f.Panel.Roster[0].Model, f.Panel.Roster[0].Lens = "model\x1b[31m", "hostile"
		if err := panel.WriteSnapshot(f.Dir, f.Panel); err != nil {
			t.Fatal(err)
		}
		out := promptRun(t, root, "R1")
		if strings.ContainsRune(out, '\x1b') || !strings.Contains(out, "model\\x1b[31m") || !strings.Contains(out, "focus\\x1b[31m") {
			t.Fatalf("unsafe prompt strings:\n%s", out)
		}
	})
}

func promptSubmit(t *testing.T, f paneltest.Fixture, slot, status, text string) {
	t.Helper()
	v := paneltest.Verdict("REQUEST_CHANGES")
	v.Slot, v.Round, v.Status = slot, f.Panel.Round, status
	if slot == "R2" {
		v.Model = "other-model"
	}
	v.ConcreteChangesRequired = []panel.ChangeRequired{{Text: text, Basis: "traced"}}
	target := f.Target()
	target.Slot = slot
	if _, err := panel.Submit(target, paneltest.Bytes(t, v), paneltest.Resolve); err != nil {
		t.Fatal(err)
	}
}

func TestPanelPrompt_PriorRound(t *testing.T) {
	root, f := promptFixture(t, 2, "")
	promptSubmit(t, f, "R1", "draft", "old superseded item")
	promptSubmit(t, f, "R1", "final", "X prior item\x1b[31m")
	promptSubmit(t, f, "R2", "final", "other slot item")
	f.Panel.Round = 2
	if err := panel.WriteSnapshot(f.Dir, f.Panel); err != nil {
		t.Fatal(err)
	}
	f.Round(t, 2)
	for _, status := range []string{"draft", "final"} {
		promptSubmit(t, f, "R1", status, "Y current item")
		out := promptRun(t, root, "R1")
		if !strings.Contains(out, "X prior item\\x1b[31m") || strings.Contains(out, "Y current item") || strings.Contains(out, "other slot item") || strings.Contains(out, "old superseded item") || strings.ContainsRune(out, '\x1b') {
			t.Fatalf("wrong prior-round selection after %s:\n%s", status, out)
		}
		for _, grade := range []string{"ADDRESSED", "PARTIAL", "MISSED", "NEW_ISSUE"} {
			if !strings.Contains(out, grade) {
				t.Errorf("missing prior-round grade %s", grade)
			}
		}
		other := promptRun(t, root, "R2")
		if !strings.Contains(other, "1. other slot item\n") || strings.Contains(other, "X prior item") || strings.Contains(other, "Y current item") {
			t.Fatalf("wrong prior-round items for the other slot:\n%s", other)
		}
	}
}

func TestPanelPrompt_PriorRoundUnavailable(t *testing.T) {
	for _, tc := range []struct{ name, reason string }{
		{"absent", "file is absent"},
		{"digest", "file digest differs from the latest submission"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, f := promptFixture(t, 2, "")
			promptSubmit(t, f, "R1", "final", "verified prior item")
			path := filepath.Join(f.Dir, "R1-round-1.json")
			if tc.name == "absent" {
				if err := os.Rename(path, filepath.Join(t.TempDir(), "saved-verdict.json")); err != nil {
					t.Fatal(err)
				}
			} else {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				subjectCLIWrite(t, f.Dir, "R1-round-1.json", strings.Replace(string(data), "verified prior item", "untrusted prior item", 1))
			}
			f.Panel.Round = 2
			if err := panel.WriteSnapshot(f.Dir, f.Panel); err != nil {
				t.Fatal(err)
			}
			f.Round(t, 2)
			out := promptRun(t, root, "R1")
			want := "Prior-round items could not be read: " + tc.reason + ".\n"
			if !strings.Contains(out, "## Prior-round items\n\n"+want) || strings.Contains(out, "no prior-round items") || strings.Contains(out, "verified prior item") || strings.Contains(out, "untrusted prior item") {
				t.Fatalf("missing prior-round read warning:\n%s", out)
			}
		})
	}
}

func TestPanelPrompt_PriorRoundV1(t *testing.T) {
	for _, tc := range []struct{ name, file, want string }{
		{"parsed", `{"verdict":"REQUEST_CHANGES","concrete_changes_required":["legacy item\u001b[31m"]}`, "legacy item\\x1b[31m"},
		{"absent", "", "no prior-round items"},
		{"unparseable", "{", "no prior-round items"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, f := promptFixture(t, 0, "")
			// WriteSnapshot is the fixture API. The legacy verdict is not a
			// submission: the test specifically exercises the pre-v2 reader.
			if tc.file != "" {
				subjectCLIWrite(t, f.Dir, "R1-round-1.json", tc.file)
			}
			subjectCLIWrite(t, f.Dir, "R2-round-1.json", `{"verdict":"REQUEST_CHANGES","concrete_changes_required":["other slot"]}`)
			f.Panel.Round = 2
			if err := panel.WriteSnapshot(f.Dir, f.Panel); err != nil {
				t.Fatal(err)
			}
			f.Round(t, 2)
			out := promptRun(t, root, "R1")
			if !strings.Contains(out, tc.want) || strings.Contains(out, "other slot") || strings.ContainsRune(out, '\x1b') {
				t.Fatalf("wrong legacy prior items:\n%s", out)
			}
		})
	}
}

func TestPanelPrompt_FromSlotTree(t *testing.T) {
	root, f := promptFixture(t, 2, "")
	layout, err := panel.NewSlotLayout(root, ".mindspec-scratch", "test", "p", 1, "R1")
	if err != nil {
		t.Fatal(err)
	}
	if err := panel.WriteSlotMetadata(layout, panel.SlotMetadata{Spec: "test", Slug: "p", Round: 1, Slot: "R1", MainRoot: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(layout.Tree, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	paneltest.Seed(t, f.Dir, f.Launch("dispatched", 1, 2))
	withTestChdir(t, filepath.Join(layout.Tree, "sub"))
	out := promptRun(t, root, "R1")
	if !strings.Contains(out, "Review this fixture.") || !strings.Contains(out, filepath.Join(layout.Dir, "attempt-2/verdict.json")) {
		t.Fatalf("slot root was not resolved:\n%s", out)
	}
}
