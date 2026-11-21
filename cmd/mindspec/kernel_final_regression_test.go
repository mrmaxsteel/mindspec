package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
)

var finalKernelBuild struct {
	sync.Once
	bytes []byte
	err   error
}

func finalKernelBinary(t *testing.T) string {
	t.Helper()
	finalKernelBuild.Do(func() { finalKernelBuild.bytes, finalKernelBuild.err = os.ReadFile(buildMindspecBinary(t)) })
	if finalKernelBuild.err != nil {
		t.Fatal(finalKernelBuild.err)
	}
	path := filepath.Join(t.TempDir(), "mindspec")
	if err := os.WriteFile(path, finalKernelBuild.bytes, 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

// finalKernelFixture uses the built command and a serialized, isolated tracker.
func finalKernelFixture(t *testing.T) *kernelE2E {
	t.Helper()
	f := kerneltest.Enrolled(t, kerneltest.WithFake(false))
	t.Setenv("MINDSPEC_TEST_FULL_BD", "1")
	f.Fake.Records = map[string]kernel.Record{}
	f.Must(os.MkdirAll(filepath.Join(f.Root, ".beads"), 0755))
	f.Must(os.WriteFile(filepath.Join(f.Root, ".beads/config.yaml"), []byte("types.custom: gate,step\n"), 0600))
	actorCmdStore(t, f)
	e := newKernelE2EWithFixture(t, finalKernelBinary(t), f, 1)
	item := e.f.Items[0]
	e.write(e.cwd, ".mindspec/domains/workflow/OWNERSHIP.yaml", "paths:\n  - internal/widget/**\n")
	e.write(e.cwd, ".mindspec/domains/workflow/architecture.md", "Workflow documentation.\n")
	e.git(e.cwd, "add", ".")
	e.git(e.cwd, "commit", "-qm", "content context")
	tree := filepath.Join(e.f.Root, ".worktrees", "worktree-"+item)
	e.git(e.f.Root, "worktree", "add", "-b", "bead/"+item, tree, "spec/"+e.f.SpecID)
	return e
}

func TestS134Final_GateCloseNeedsPanelEvidence(t *testing.T) {
	for _, mode := range []string{"path", "retally", "tally-label", "orphan-panel-file", "gate-attempt", "final-only"} {
		t.Run(mode, func(t *testing.T) {
			e := finalKernelFixture(t)
			e.apply()
			item := e.f.Items[0]
			out := e.implement(0)
			gate := item + ".review"
			if mode == "final-only" {
				e.create(out)
				e.review("APPROVE")
				e.ok("complete", item)
				gate = e.f.Epic + ".final"
			}
			head := e.git(e.cwd, "rev-parse", "HEAD")
			path := ".mindspec/specs/001-x/spec.md"
			if mode == "orphan-panel-file" {
				path = ".mindspec/specs/001-x/reviews/unregistered/verdict.json"
				head = e.commit(e.proof, path, `{"kind":"passed"}`)
			}
			blob := e.git(e.cwd, "rev-parse", head+":"+path)
			receipt := kernel.Receipt{Kind: "passed", Path: path, Commit: head, Blob: blob}
			if mode == "retally" || mode == "gate-attempt" {
				receipt = kernel.Receipt{Kind: "passed", Retally: "forged"}
			}
			if mode == "tally-label" {
				receipt = kernel.Receipt{Kind: "passed", Retally: "passing-tally", Subject: head}
			}
			if mode != "final-only" {
				e.git(e.cwd, "merge", "--no-ff", "-m", "raw merge", "bead/"+item)
			}
			forge := func(id string) {
				raw, err := json.Marshal(kernel.Metadata(kernel.KeyClose, receipt))
				e.must(err)
				e.tracker("update", e.f.ID(id), "--metadata", string(raw))
				e.tracker("close", e.f.ID(id))
			}
			forge(gate)
			if mode == "gate-attempt" {
				e.apply()
				gate += ".a2"
				forge(gate)
			}
			if mode != "final-only" {
				e.tracker("close", item)
			}
			raw, _ := e.run("next-action", e.f.SpecID, "--json")
			var doc kernel.Output
			e.must(json.Unmarshal([]byte(strings.Split(raw, "\n")[0]), &doc))
			flagged := false
			for _, flag := range doc.Flags {
				flagged = flagged || (flag.Code == "unreceipted-close" && flag.Step == gate)
			}
			if !flagged {
				t.Fatalf("forged gate has no unreceipted-close flag: %s", raw)
			}
			if mode != "final-only" {
				e.refuse("gate", "complete", item)
				if value := e.row(item).Metadata[kernel.KeyCompletion]; value != nil {
					t.Fatalf("refusal wrote a completed subject: %v", value)
				}
				forge(e.f.Epic + ".final")
			}
			e.refuse("unreceipted-close", "impl", "approve", e.f.SpecID)
			if e.row(e.f.Epic).Status == "closed" {
				t.Fatal("forged gate finalized the epic")
			}
		})
	}
}

func TestS134Final_SlotRetryVerbs(t *testing.T) {
	for _, mode := range []string{"submit", "panel-submit", "request-changes", "reject", "missing-from", "wrong-slot", "wrong-round", "wrong-gate", "valid-original", "multi-hop"} {
		t.Run(mode, func(t *testing.T) {
			e := finalKernelFixture(t)
			e.apply()
			e.create(e.implement(0))
			slots := e.apply()
			var last kernel.Action
			for _, a := range slots.Actions {
				if strings.HasSuffix(a.Step, ".R3") {
					last = a
					continue
				}
				token := e.lease(a.Step)
				e.ok("submit", a.Step, "--lease", token, "--artifact", e.verdict(a, "APPROVE"))
			}
			if last.Step == "" {
				t.Fatal("no third slot")
			}
			e.tracker("close", e.f.ID(last.Step))
			e.apply()
			retry := last.Step + ".a2"
			if mode == "multi-hop" {
				e.tracker("close", e.f.ID(retry))
				e.apply()
				retry = last.Step + ".a3"
			}
			bad := false
			if mode == "missing-from" || mode == "wrong-slot" || mode == "wrong-round" || mode == "wrong-gate" || mode == "valid-original" {
				bad = true
				from := last.Step
				if mode == "missing-from" {
					from = ""
				}
				if mode == "wrong-slot" {
					from = strings.TrimSuffix(last.Step, "R3") + "R2"
				}
				if mode == "wrong-round" {
					from = strings.Replace(last.Step, ".r1.", ".r2.", 1)
				}
				if mode == "wrong-gate" {
					from = e.f.Epic + ".final.r1.R3"
				}
				raw, err := json.Marshal(kernel.Metadata(kernel.KeyFrom, from))
				e.must(err)
				e.tracker("update", e.f.ID(retry), "--metadata", string(raw))
				if mode == "valid-original" {
					raw, err = json.Marshal(kernel.Metadata(kernel.KeyClose, kernel.Receipt{Kind: "passed", Retally: "already-voted"}))
					e.must(err)
					e.tracker("update", e.f.ID(last.Step), "--metadata", string(raw))
				}
			}
			token := e.lease(retry)
			a := last
			a.Step = retry
			word := "APPROVE"
			if mode == "request-changes" {
				word = "REQUEST_CHANGES"
			}
			if mode == "reject" {
				word = "REJECT"
			}
			path := e.verdict(a, word)
			args := []string{"submit", retry, "--lease", token, "--artifact", path}
			if mode == "panel-submit" {
				r := e.row(retry)
				reg, _, err := kernel.Decode[kernel.RegistrationRecord](e.row(r.Parent).Metadata, kernel.KeyRegistration)
				e.must(err)
				args = []string{"panel", "submit", reg.Slug, "--spec", e.f.SpecID, "--slot", "R3", "--round", "1", "--lease", token, "--file", path}
			}
			raw, code := e.run(args...)
			if bad {
				if code == 0 {
					t.Fatalf("invalid lineage accepted: %s", raw)
				}
				return
			}
			if code != 0 {
				t.Fatalf("retry refused: %d %s", code, raw)
			}
			out := e.apply()
			for _, flag := range out.Flags {
				if flag.Code == "unreceipted-close" {
					t.Fatalf("repair flag remains: %+v", flag)
				}
			}
			if mode == "request-changes" {
				if e.row(e.f.Items[0]+".review.r2.fix").Status != "open" {
					t.Fatal("retry did not release fix")
				}
				return
			}
			if mode == "reject" {
				if len(out.Decisions) != 1 {
					t.Fatalf("retry did not release decision: %+v", out)
				}
				return
			}
			complete := false
			for _, run := range out.Run {
				complete = complete || (len(run.Argv) > 1 && run.Argv[1] == "complete")
			}
			if !complete {
				t.Fatalf("retry did not pass gate: %+v", out)
			}
		})
	}
}

func TestS134Final_CheckAndPrintedSubmit(t *testing.T) {
	e := finalKernelFixture(t)
	out := e.apply()
	var action kernel.Action
	for _, a := range out.Actions {
		if a.Kind == "implement" {
			action = a
		}
	}
	item := e.f.Items[0]
	for _, kind := range []string{"implement", "fix"} {
		if action.Kind != kind {
			t.Fatalf("no %s action: %+v", kind, action)
		}
		token := e.lease(action.Step)
		head := e.commit(e.tree(item), kind+".txt", "implemented\n")
		before := kerneltest.Snapshot(t, e.f)
		data, err := e.f.BinaryCommandIn(e.tree(item), e.bin, "complete", item, "--check")
		if err != nil {
			t.Fatalf("%s check refused: %v %s", kind, err, data)
		}
		if before != kerneltest.Snapshot(t, e.f) {
			t.Fatal("check wrote state, receipts, or refs")
		}
		e.refuse("gate", "complete", item)
		argv := append([]string{}, action.Submit[1:]...)
		for i, arg := range argv {
			switch arg {
			case "<token>":
				argv[i] = token
			case "<commit>":
				argv[i] = head
			}
		}
		raw := e.ok(argv...)
		var submitted kernel.Output
		e.must(json.Unmarshal([]byte(raw), &submitted))
		if submitted.Revision == "" {
			t.Fatal("submit omitted revision")
		}
		for _, a := range submitted.Actions {
			if !a.Exists {
				t.Fatalf("created action reported absent: %+v", a)
			}
		}
		if kind == "implement" {
			replay := e.ok(argv...)
			var again kernel.Output
			e.must(json.Unmarshal([]byte(replay), &again))
			if len(again.Run) == 0 || again.Status != "work" {
				t.Fatalf("accepted retry misreports state: %s", replay)
			}
			e.create(submitted)
			reviewed := e.review("REQUEST_CHANGES")
			currentRaw, _ := e.run("next-action", e.f.SpecID, "--json")
			var current kernel.Output
			e.must(json.Unmarshal([]byte(strings.Split(currentRaw, "\n")[0]), &current))
			if reviewed.Revision != current.Revision {
				t.Fatalf("submit revision differs from durable state: %s != %s", reviewed.Revision, current.Revision)
			}
			for _, a := range e.apply().Actions {
				if a.Kind == "fix" {
					action = a
				}
			}
		}
	}
	// A source change with no domain documentation must still fail.
	e.write(e.tree(item), "internal/widget/widget.go", "package widget\nconst Value = 1\n")
	before := kerneltest.Snapshot(t, e.f)
	data, err := e.f.BinaryCommandIn(e.tree(item), e.bin, "complete", item, "--check")
	if err == nil || !strings.Contains(string(data), "doc-sync:") {
		t.Fatalf("content failure passed check: %v %s", err, data)
	}
	if before != kerneltest.Snapshot(t, e.f) {
		t.Fatal("failed check wrote state")
	}
}
