package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/panel"
)

func rebindGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/usr/bin/false",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func rebindWrite(t *testing.T, root, name, content string) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupRebind(t *testing.T) (root, dir, from string) {
	t.Helper()
	root = mkPanelTestRoot(t, "")
	var err error
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	withTestChdir(t, root)
	for _, name := range []string{"reason", "spec"} {
		if err := panelRebindCmd.Flags().Set(name, ""); err != nil {
			t.Fatal(err)
		}
		panelRebindCmd.Flags().Lookup(name).Changed = false
	}
	t.Cleanup(func() {
		for _, name := range []string{"reason", "spec"} {
			_ = panelRebindCmd.Flags().Set(name, "")
			panelRebindCmd.Flags().Lookup(name).Changed = false
		}
	})
	rebindGit(t, root, "init", "-q", "-b", "main")
	rebindWrite(t, root, "source.go", "package example\n")
	rebindWrite(t, root, ".gitignore", "review/\n")
	rebindWrite(t, root, ".mindspec/domains/core/OWNERSHIP.yaml", "paths: []\n")
	rebindGit(t, root, "add", ".")
	rebindGit(t, root, "commit", "-qm", "base")
	from = rebindGit(t, root, "rev-parse", "HEAD")
	digest, err := panel.SubjectDigest(rebindGit(t, root, "ls-tree", "-r", "HEAD")+"\n", panel.Subject("final_review", ".mindspec/specs/900-demo"))
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.Join(root, "review", "demo")
	err = panel.Create(dir, panel.CreateInput{Spec: "900-demo", Target: "main", Gate: "final_review", Round: 1,
		ExpectedReviewers: 2, ReviewedHeadSHA: from, SubjectDigest: digest,
		Roster: []panel.RosterSlot{{Slot: "R1", Model: "codex", Lens: "correctness"}, {Slot: "R2", Model: "claude", Lens: "coverage"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range []string{"R1", "R2"} {
		rebindWrite(t, dir, slot+"-round-1.json", `{"verdict":"APPROVE"}`)
	}
	return
}

func rebindRegistration(t *testing.T, dir string) panel.Registration {
	t.Helper()
	res, err := panel.Tally(dir)
	if err != nil || res.PanelErr != nil || res.Panel == nil {
		t.Fatalf("read panel: %+v %v", res, err)
	}
	return panel.Registration{Dir: dir, Panel: *res.Panel}
}

func rebindGate(t *testing.T, root, dir string) panel.Decision {
	t.Helper()
	reg := rebindRegistration(t, dir)
	return panel.PanelGateDecision(resolvePanelGateFacts(root, reg))
}

func TestPanelRebind_OwnershipOnly(t *testing.T) {
	root, dir, from := setupRebind(t)
	name := ".mindspec/domains/core/OWNERSHIP.yaml"
	rebindWrite(t, root, name, "paths: [internal/]\n")
	rebindGit(t, root, "add", name)
	rebindGit(t, root, "commit", "-qm", "ownership")
	if got := rebindGit(t, root, "diff", "--name-only", from+"..HEAD"); got != name {
		t.Fatalf("fixture diff %q", got)
	}
	assertRebindSuccess(t, root, dir, from, []string{name})
}

func assertRebindSuccess(t *testing.T, root, dir, from string, paths []string) {
	t.Helper()
	if d := rebindGate(t, root, dir); d.Action != panel.Block || !strings.Contains(d.Message, "commits landed after review") {
		t.Fatalf("before: %+v", d)
	}
	old := rebindRegistration(t, dir).Panel
	out, err := runPanelVerbCmd("rebind", "demo", "--reason", "max: checked the diff")
	if err != nil {
		t.Fatalf("rebind: %v\n%s", err, out)
	}
	p := rebindRegistration(t, dir).Panel
	tip := rebindGit(t, root, "rev-parse", "HEAD")
	if p.Round != old.Round {
		t.Fatalf("round changed: %d -> %d", old.Round, p.Round)
	}
	if p.ReviewedHeadSHA != tip || p.SubjectDigest == old.SubjectDigest {
		t.Fatalf("binding did not advance: %+v", p)
	}
	if len(p.Rebinds) != 1 {
		t.Fatalf("rebind entries = %+v", p.Rebinds)
	}
	entry := p.Rebinds[0]
	if entry.Round != old.Round || entry.FromSHA != from || entry.ToSHA != tip || entry.Reason != "max: checked the diff" || entry.CreatedAt == "" || !reflect.DeepEqual(entry.Paths, paths) {
		t.Fatalf("entry = %+v, paths want %v", entry, paths)
	}
	lines, err := panel.ReadLedger(dir)
	if err != nil || len(lines) != 2 {
		t.Fatalf("ledger: %s %v", lines, err)
	}
	var event panel.RebindRecord
	if err := json.Unmarshal(lines[1], &event); err != nil {
		t.Fatal(err)
	}
	if event.Record != panel.RecordRebind || event.SubjectDigest != p.SubjectDigest || !reflect.DeepEqual(event.Rebind, entry) {
		t.Fatalf("event mismatch: %+v", event)
	}
	if d := rebindGate(t, root, dir); d.Action != panel.Allow {
		t.Fatalf("after: %+v", d)
	}
}

func TestPanelRebind_AnyPath(t *testing.T) {
	for _, name := range []string{"source.go", "src/reviews/source.go", "docs/space name.md", "docs/\"quoted\".md", "docs/世界.md"} {
		t.Run(name, func(t *testing.T) {
			root, dir, from := setupRebind(t)
			rebindWrite(t, root, name, "changed\n")
			rebindGit(t, root, "add", name)
			rebindGit(t, root, "commit", "-qm", "change")
			assertRebindSuccess(t, root, dir, from, []string{name})
		})
	}
}

func TestPanelRebind_AlreadyFresh(t *testing.T) {
	_, dir, _ := setupRebind(t)
	before := panelTreeBytes(t, dir)
	out, err := runPanelVerbCmd("rebind", "demo", "--reason", "max: checked")
	if err != nil || !strings.Contains(out, "already fresh") {
		t.Fatalf("%v %s", err, out)
	}
	if after := panelTreeBytes(t, dir); !reflect.DeepEqual(before, after) {
		t.Fatalf("already-fresh rebind wrote files: before=%v after=%v", before, after)
	}
}

func TestPanelRebind_Refusals(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(*testing.T, string, string)
	}{
		{"empty reason", "non-empty reason", nil},
		{"malformed", "unreadable", func(t *testing.T, root, dir string) { rebindWrite(t, dir, panel.FileName, "{") }},
		{"abandoned", "abandoned", func(t *testing.T, root, dir string) {
			reg := rebindRegistration(t, dir)
			p := reg.Panel
			p.Abandoned = true
			p.AbandonReason = "max: abandoned"
			raw, _ := json.Marshal(panel.AbandonRecord{Record: panel.RecordAbandon, Spec: p.Spec, Panel: "demo", Reason: p.AbandonReason, By: "max", CreatedAt: "2026-10-20T00:00:00Z"})
			if err := panel.AppendRecords(dir, raw); err != nil {
				t.Fatal(err)
			}
			if err := panel.WriteSnapshot(dir, p); err != nil {
				t.Fatal(err)
			}
		}},
		{"round mismatch", "round", func(t *testing.T, root, dir string) { rebindWrite(t, dir, "R1-round-2.json", `{"verdict":"APPROVE"}`) }},
		{"hand edited", "outside the panel verbs", func(t *testing.T, root, dir string) {
			p := rebindRegistration(t, dir).Panel
			p.ReviewedHeadSHA = strings.Repeat("b", 40)
			if err := panel.WriteSnapshot(dir, p); err != nil {
				t.Fatal(err)
			}
		}},
		{"incomplete", "incomplete", func(t *testing.T, root, dir string) {
			if err := os.Rename(filepath.Join(dir, "R2-round-1.json"), filepath.Join(dir, "R2.saved")); err != nil {
				t.Fatal(err)
			}
		}},
		{"reject", "REJECT", func(t *testing.T, root, dir string) { rebindWrite(t, dir, "R2-round-1.json", `{"verdict":"REJECT"}`) }},
		{"hard block", "hard_block", func(t *testing.T, root, dir string) {
			rebindWrite(t, dir, "R2-round-1.json", `{"verdict":"APPROVE","hard_block":true}`)
		}},
		{"invalid", "INVALID", func(t *testing.T, root, dir string) { rebindWrite(t, dir, "R2-round-1.json", `{"verdict":"UNKNOWN"}`) }},
		{"unresolved", "REQUEST_CHANGES", func(t *testing.T, root, dir string) {
			rebindWrite(t, dir, "R2-round-1.json", `{"verdict":"REQUEST_CHANGES","rationale":"fix this"}`)
		}},
		{"not ancestor", "ancestry", func(t *testing.T, root, dir string) {
			rebindGit(t, root, "checkout", "--orphan", "unrelated")
			rebindWrite(t, root, "other.txt", "unrelated\n")
			rebindGit(t, root, "add", ".")
			rebindGit(t, root, "commit", "-qm", "unrelated")
			rebindGit(t, root, "branch", "-f", "main", "HEAD")
		}},
		{"config", ".mindspec/config.yaml", func(t *testing.T, root, dir string) {
			rebindWrite(t, root, ".mindspec/config.yaml", "version: 1\n")
			rebindGit(t, root, "add", ".mindspec/config.yaml")
			rebindGit(t, root, "commit", "-qm", "config")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, dir, _ := setupRebind(t)
			rebindWrite(t, root, "source.go", "package changed\n")
			rebindGit(t, root, "add", "source.go")
			rebindGit(t, root, "commit", "-qm", "advance")
			if tc.mutate != nil {
				tc.mutate(t, root, dir)
			}
			reason := "max: checked"
			if tc.name == "empty reason" {
				reason = "  "
			}
			before := panelTreeBytes(t, dir)
			out, err := runPanelVerbCmd("rebind", "demo", "--reason", reason)
			if err == nil || !strings.Contains(fmt.Sprint(err), tc.want) {
				t.Fatalf("want refusal containing %q, got %v %s", tc.want, err, out)
			}
			if after := panelTreeBytes(t, dir); !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal wrote panel files")
			}
		})
	}
}

func TestPanelRebind_ExcludedPaths(t *testing.T) {
	for _, name := range []string{".beads/issues.jsonl", ".mindspec/specs/900-demo/reviews/other/BRIEF.md", ".mindspec/reviews/other/BRIEF.md", "review/other/BRIEF.md"} {
		t.Run(name, func(t *testing.T) {
			root, dir, _ := setupRebind(t)
			rebindWrite(t, root, name, "artifact\n")
			rebindGit(t, root, "add", "-f", name)
			rebindGit(t, root, "commit", "-qm", "artifact")
			out, err := runPanelVerbCmd("rebind", "demo", "--reason", "max: artifact only")
			if err != nil {
				t.Fatalf("%v %s", err, out)
			}
			p := rebindRegistration(t, dir).Panel
			if len(p.Rebinds) != 1 || len(p.Rebinds[0].Paths) != 0 {
				t.Fatalf("excluded paths recorded: %+v", p.Rebinds)
			}
			if d := rebindGate(t, root, dir); d.Action != panel.Allow {
				t.Fatalf("after: %+v", d)
			}
		})
	}
}
