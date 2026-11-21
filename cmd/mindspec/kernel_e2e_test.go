package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

type kernelE2E struct {
	t                                      *testing.T
	f                                      *kerneltest.Fixture
	bin, proof, cwd, note, bug, unlabelled string
	seen                                   map[string]bool
}

// Retain the built bytes because the first caller's TempDir can be cleaned while
// other parallel tests still run. Each caller gets an executable with its lifetime.
var kernelE2EBuild struct {
	sync.Once
	data []byte
	err  error
}

func kernelE2EBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("built-binary lifecycle runs in the explicit end-to-end suite")
	}
	kernelE2EBuild.Do(func() {
		kernelE2EBuild.data, kernelE2EBuild.err = os.ReadFile(buildMindspecBinary(t))
	})
	if kernelE2EBuild.err != nil {
		t.Fatal(kernelE2EBuild.err)
	}
	path := filepath.Join(t.TempDir(), "mindspec")
	if err := os.WriteFile(path, kernelE2EBuild.data, 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func newKernelE2E(t *testing.T, bin string, sizes ...int) *kernelE2E {
	t.Helper()
	return newKernelE2EWithFixture(t, bin, kerneltest.EmptyRepo(t), sizes...)
}

func newKernelE2EWithFixture(t *testing.T, bin string, f *kerneltest.Fixture, sizes ...int) *kernelE2E {
	t.Helper()
	return newKernelE2EWithLayout(t, bin, f, ".mindspec", sizes...)
}

func newKernelE2EWithLayout(t *testing.T, bin string, f *kerneltest.Fixture, docs string, sizes ...int) *kernelE2E {
	t.Helper()
	e := &kernelE2E{t: t, f: f, bin: bin}
	root := f.Root
	t.Logf("sandbox %s", root)
	size := 3
	if len(sizes) > 0 {
		size = sizes[0]
	}
	f.SpecID = "001-x"
	f.SpecDir = filepath.Join(root, docs, "specs", f.SpecID)
	e.git(root, "config", "user.name", "Kernel Test")
	e.git(root, "config", "user.email", "kernel@example.invalid")
	e.git(root, "config", "core.hooksPath", "/dev/null")
	e.git(root, "branch", "-M", "main")
	e.write(root, ".gitignore", ".worktrees/\n.mindspec/phase.lock\n.beads/*\n!.beads/config.yaml\n!.beads/issues.jsonl\n")
	e.write(root, ".mindspec/config.yaml", "enforcement:\n  panel_gate: false\nauto_open_finalize_pr: false\npanel:\n  reviewers:\n    - family: harness\n      model: test-model\n      lens: correctness\n      count: 3\n")
	e.write(root, filepath.Join(docs, "adr", "ADR-9001.md"), "# ADR-9001: Fixture\n\n- **Status**: Accepted\n- **Domain(s)**: workflow\n\n## Decision\nExercise the local kernel.\n")
	e.write(root, ".beads/issues.jsonl", "")
	e.git(root, "add", "-f", ".beads/config.yaml", ".beads/issues.jsonl")
	e.git(root, "add", ".")
	e.git(root, "commit", "-qm", "fixture")
	branch, err := workspace.SpecBranch(f.SpecID)
	e.must(err)
	e.git(root, "branch", branch)
	name, err := workspace.SpecWorktreeName(f.SpecID)
	e.must(err)
	e.cwd = filepath.Join(root, ".worktrees", name)
	e.git(root, "worktree", "add", e.cwd, branch)
	f.SpecDir = filepath.Join(e.cwd, docs, "specs", f.SpecID)
	e.write(e.cwd, filepath.Join(docs, "specs", f.SpecID, "spec.md"), "---\nstatus: Approved\n---\n# Example\n\n## Goal\nExercise the kernel.\n\n## Impacted Domains\n- workflow\n")
	e.write(e.cwd, filepath.Join(docs, "specs", f.SpecID, "plan.md"), strings.Replace(kerneltest.EnrolmentPlan(f.SpecID, size), "version:", "adr_citations: [ADR-9001]\nversion:", 1))
	e.git(e.cwd, "add", ".")
	e.git(e.cwd, "commit", "-qm", "spec")
	var epic kernel.Record
	e.must(json.Unmarshal(e.tracker("create", "--title", "[SPEC 001-x] Example", "--type", "epic", "--metadata", `{"spec_id":"001-x","mindspec_phase":"plan"}`, "--json"), &epic))
	f.Epic = epic.ID
	f.Store = kernel.NewStore(root, f.Epic, kerneltest.Runner(t, root))
	planOutput := e.ok("plan", "approve", f.SpecID)
	if !strings.Contains(planOutput, "mindspec next-action "+shellQuoteTarget(f.SpecID)+" --apply") || strings.Contains(planOutput, "Plan is approved. Run `mindspec next`") || strings.Contains(planOutput, "**No active worktree.** Run `mindspec next`") {
		t.Fatalf("new approval did not direct the kernel loop: %s", planOutput)
	}
	data, err := os.ReadFile(filepath.Join(f.SpecDir, "plan.md"))
	e.must(err)
	fm, err := validate.ParsePlanFrontmatter(string(data))
	e.must(err)
	f.Items = fm.BeadIDs
	if len(f.Items) != size {
		t.Fatalf("enrolment items: %v", f.Items)
	}
	e.proof = filepath.Join(t.TempDir(), "evidence")
	e.git(root, "worktree", "add", "--detach", e.proof, branch)
	return e
}
func (e *kernelE2E) must(err error) {
	e.t.Helper()
	if err != nil {
		e.t.Fatal(err)
	}
}
func (e *kernelE2E) write(root, path, body string) {
	e.t.Helper()
	p := filepath.Join(root, path)
	e.must(os.MkdirAll(filepath.Dir(p), 0755))
	e.must(os.WriteFile(p, []byte(body), 0600))
}
func (e *kernelE2E) git(root string, args ...string) string {
	e.t.Helper()
	out, err := e.f.Command("git", append([]string{"-C", root}, args...)...)
	if err != nil {
		e.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func (e *kernelE2E) tracker(args ...string) []byte {
	e.t.Helper()
	out, err := e.f.Command("bd", args...)
	if err != nil {
		e.t.Fatalf("sandbox tracker %v: %v %s", args, err, out)
	}
	return out
}
func (e *kernelE2E) run(args ...string) (string, int) {
	e.t.Helper()
	e.t.Logf("binary %v", args[:min(len(args), 3)])
	key := args[0]
	if key == "panel" || key == "impl" {
		key += " " + args[1]
	}
	before := ""
	if e.seen != nil && !e.seen[key] {
		before = e.protectedSnapshot()
	}
	defer func() {
		if before != "" {
			after := e.protectedSnapshot()
			if before != after {
				e.t.Fatalf("%s changed the foreign snapshot\nbefore: %s\nafter: %s", key, before, after)
			}
		}
	}()
	cwd := e.cwd
	if _, err := os.Stat(cwd); err != nil {
		cwd = e.f.Root
	}
	out, err := e.f.BinaryCommandIn(cwd, e.bin, args...)
	if err == nil {
		if before != "" {
			e.seen[key] = true
		}
		return string(out), 0
	}
	if errors.Is(err, context.DeadlineExceeded) {
		e.t.Fatalf("binary %v timed out: %v\n%s", args[:min(len(args), 3)], err, out)
	}
	var ex *exec.ExitError
	if !errors.As(err, &ex) {
		e.t.Fatal(err)
	}
	return string(out), ex.ExitCode()
}
func (e *kernelE2E) ok(args ...string) string {
	e.t.Helper()
	out, code := e.run(args...)
	if code != 0 {
		e.t.Fatalf("%v: exit %d\n%s", args[:min(len(args), 3)], code, out)
	}
	return out
}
func (e *kernelE2E) refuse(word string, args ...string) string {
	e.t.Helper()
	out, code := e.run(args...)
	if code == 0 || !strings.Contains(out, word) {
		e.t.Fatalf("expected refusal %q for %v: %d %s", word, args, code, out)
	}
	return out
}
func (e *kernelE2E) apply() kernel.Output {
	e.t.Helper()
	raw, code := e.run("next-action", e.f.SpecID, "--apply", "--json")
	if code != 0 && code != 3 && code != 5 {
		e.t.Fatalf("apply: %d %s", code, raw)
	}
	var out kernel.Output
	e.must(json.Unmarshal([]byte(strings.Split(raw, "\n")[0]), &out))
	return out
}
func (e *kernelE2E) lease(id string, extra ...string) string {
	e.t.Helper()
	raw := e.ok(append([]string{"lease", id}, extra...)...)
	parts := strings.Fields(raw)
	if len(parts) != 3 {
		e.t.Fatalf("lease output: %s", raw)
	}
	return parts[2]
}
func (e *kernelE2E) row(id string) kernel.Record {
	e.t.Helper()
	r, err := e.f.Store.Lookup(id)
	e.must(err)
	if r == nil {
		e.t.Fatalf("missing %s", id)
	}
	return *r
}
func (e *kernelE2E) tree(item string) string {
	e.t.Helper()
	branch, err := workspace.BeadBranch(item)
	e.must(err)
	raw := e.git(e.f.Root, "worktree", "list", "--porcelain")
	for _, block := range strings.Split(raw, "\n\n") {
		if strings.Contains(block, "branch refs/heads/"+branch) {
			return strings.TrimPrefix(strings.Split(block, "\n")[0], "worktree ")
		}
	}
	e.t.Fatalf("no tree for %s: %s", item, raw)
	return ""
}
func (e *kernelE2E) commit(root, path, body string) string {
	e.t.Helper()
	e.write(root, path, body)
	e.git(root, "add", "--", path)
	e.git(root, "commit", "-qm", "evidence")
	return e.git(root, "rev-parse", "HEAD")
}
func (e *kernelE2E) implement(i int) kernel.Output {
	e.t.Helper()
	item := e.f.Items[i]
	step := item + ".impl.a1"
	token := e.lease(step)
	head := e.commit(e.tree(item), fmt.Sprintf("item-%d.txt", i), "implemented\n")
	raw := e.ok("submit", step, "--lease", token, "--commit", head)
	var out kernel.Output
	e.must(json.Unmarshal([]byte(raw), &out))
	return out
}
func (e *kernelE2E) create(out kernel.Output) {
	e.t.Helper()
	if len(out.Run) == 0 || len(out.Run[0].Argv) < 3 || out.Run[0].Argv[1] != "panel" {
		e.t.Fatalf("no panel command: %+v", out)
	}
	e.ok(out.Run[0].Argv[1:]...)
}
func (e *kernelE2E) verdict(a kernel.Action, word string) string {
	e.t.Helper()
	r := e.row(a.Step)
	tuple, _, err := r.Tuple()
	e.must(err)
	reg, ok, err := kernel.Decode[kernel.RegistrationRecord](e.row(r.Parent).Metadata, kernel.KeyRegistration)
	e.must(err)
	if !ok {
		e.t.Fatalf("no registration for %s", a.Step)
	}
	v := paneltest.Verdict(word)
	v.Slot = fmt.Sprintf("R%d", tuple.Slot)
	v.Round = a.Round
	v.ReviewedSHA = a.Subject.Commit
	v.Model = "test-model"
	path := filepath.Join(".mindspec/specs", e.f.SpecID, "reviews", reg.Slug, fmt.Sprintf("verdict-%d-%s.json", a.Round, v.Slot))
	e.commit(e.proof, path, string(paneltest.Bytes(e.t, v)))
	return filepath.Join(e.proof, path)
}
func (e *kernelE2E) review(word string) kernel.Output {
	e.t.Helper()
	out := e.apply()
	n := 0
	var latest kernel.Output
	for _, a := range out.Actions {
		if a.Kind != "review-slot" {
			continue
		}
		token := e.lease(a.Step)
		path := e.verdict(a, word)
		raw := e.ok("submit", a.Step, "--lease", token, "--artifact", path)
		var submitted kernel.Output
		e.must(json.Unmarshal([]byte(raw), &submitted))
		latest = submitted
		if submitted.Revision == "" {
			e.t.Fatal("submit omitted its revision")
		}
		for _, next := range submitted.Actions {
			if !next.Exists {
				e.t.Fatalf("created step reported absent: %+v", next)
			}
		}
		n++
	}
	if n != 3 {
		e.t.Fatalf("review slots %d: %+v", n, out)
	}
	return latest
}

func (e *kernelE2E) judge(value string, n int) {
	e.t.Helper()
	out := e.apply()
	var action kernel.Action
	for _, a := range out.Actions {
		if a.Kind == "change-judge" {
			action = a
			break
		}
	}
	if action.Step == "" {
		e.t.Fatalf("no judge: %+v", out)
	}
	s := e.f.State()
	tuple, _, err := s.Records[action.Step].Tuple()
	e.must(err)
	old, err := kernel.AcceptedSubject(s, tuple.Gate)
	e.must(err)
	token := e.lease(action.Step)
	reg, ok, err := kernel.Decode[kernel.RegistrationRecord](s.Records[tuple.Gate].Metadata, kernel.KeyRegistration)
	e.must(err)
	if !ok {
		e.t.Fatal("judge has no passing panel")
	}
	j := kernel.Judgement{Value: value, OldSubject: old, NewSubject: action.Subject.Commit, Reason: "The review covers this change."}
	if value == "re-panel" {
		j.SlotCount = n
		j.Paths = []string{".mindspec/config.yaml"}
		j.Reason = ""
	}
	data, err := json.Marshal(j)
	e.must(err)
	path := filepath.Join(".mindspec/specs", e.f.SpecID, "reviews", reg.Slug, "judgement-"+value+".json")
	e.commit(e.proof, path, string(data))
	e.ok("submit", action.Step, "--lease", token, "--artifact", filepath.Join(e.proof, path))
}
func (e *kernelE2E) foreign() {
	e.t.Helper()
	for _, entry := range []struct {
		title, typ string
		id         *string
		parent     string
	}{{"foreign note", "task", &e.note, e.f.Epic}, {"foreign bug", "bug", &e.bug, ""}} {
		args := []string{"create", "--title", entry.title, "--type", entry.typ, "--json"}
		if entry.parent != "" {
			args = append(args, "--parent", entry.parent)
		}
		var r kernel.Record
		e.must(json.Unmarshal(e.tracker(args...), &r))
		*entry.id = r.ID
	}
	if len(e.f.Items) > 1 {
		e.tracker("dep", "add", e.f.Items[1], e.bug, "--type", "blocks")
	}
	e.tracker("update", kerneltest.ID(e.t, e.f, e.f.Items[0]+".impl.a1"), "--claim")
	e.seen = map[string]bool{}
	out := e.apply()
	for _, a := range out.Actions {
		if a.Step == e.note {
			e.t.Fatal("foreign note became a kernel action")
		}
	}
	labelled := false
	for _, label := range e.row(e.note).Labels {
		labelled = labelled || label == "mindspec"
	}
	if !labelled {
		e.t.Fatal("plain child did not inherit the epic label")
	}
	raw, err := json.Marshal(out)
	e.must(err)
	for _, flag := range []string{"raw-claim", "foreign-child-open"} {
		if !strings.Contains(string(raw), flag) {
			e.t.Fatalf("missing %s: %s", flag, raw)
		}
	}
}
func (e *kernelE2E) ready() {
	e.t.Helper()
	owned := map[string]bool{e.f.Epic: true}
	for _, id := range e.f.Items {
		owned[id] = true
	}
	rows, err := e.f.Store.All()
	e.must(err)
	for _, r := range rows {
		tuple, ok, err := r.Tuple()
		parsed, parseErr := kernelid.Parse(r.Key())
		if !owned[r.ID] && !(err == nil && ok && parseErr == nil && parsed == tuple && tuple.Epic == e.f.Epic) {
			continue
		}
		labelled := false
		for _, label := range r.Labels {
			labelled = labelled || label == "mindspec"
		}
		if !strings.HasPrefix(r.Title, "[mindspec]") || !labelled {
			e.t.Fatalf("owned record missing view markers: %+v", r)
		}
	}

	var ready []kernel.Record
	e.must(json.Unmarshal(e.tracker("ready", "--json", "--limit", "0"), &ready))
	for _, r := range ready {
		if owned[r.ID] && !strings.HasPrefix(r.Title, "[mindspec]") {
			e.t.Fatalf("unmarked title: %+v", r)
		}
	}
	ready = nil
	e.must(json.Unmarshal(e.tracker("ready", "--exclude-label", "mindspec", "--json", "--limit", "0"), &ready))
	found := map[string]bool{}
	for _, r := range ready {
		if r.ID != e.unlabelled && r.ID != e.bug {
			e.t.Fatalf("owned record in foreign view: %+v", r)
		}
		found[r.ID] = true
	}
	if !found[e.unlabelled] || !found[e.bug] {
		e.t.Fatalf("foreign view missing rows: %+v; note: %s", ready, e.tracker("show", e.note, "--json"))
	}
}

// Preserve foreign bytes and edges. Dependency lists expand their target rows;
// the fields of a computed target may change, but its edge identity may not.
func (e *kernelE2E) protectedSnapshot() string {
	e.t.Helper()
	raw := kerneltest.Snapshot(e.t, e.f)
	dec := json.NewDecoder(strings.NewReader(raw))
	var rawRows []json.RawMessage
	e.must(dec.Decode(&rawRows))
	rows := make([]kernel.Record, len(rawRows))
	owned := map[string]bool{e.f.Epic: true}
	for _, id := range e.f.Items {
		owned[id] = true
	}
	for i, raw := range rawRows {
		e.must(json.Unmarshal(raw, &rows[i]))
		r := rows[i]
		tuple, ok, err := r.Tuple()
		parsed, parseErr := kernelid.Parse(r.Key())
		if err == nil && ok && parseErr == nil && parsed == tuple && tuple.Epic == e.f.Epic {
			owned[r.ID] = true
		}
	}
	kept := []json.RawMessage{}
	links := map[string][]json.RawMessage{}
	for index, r := range rows {
		var deps []json.RawMessage
		e.must(dec.Decode(&deps))
		if !owned[r.ID] {
			kept = append(kept, rawRows[index])
			for _, raw := range deps {
				var d struct {
					ID   string `json:"id"`
					Type string `json:"dependency_type"`
				}
				e.must(json.Unmarshal(raw, &d))
				if owned[d.ID] {
					encoded, err := json.Marshal(d)
					e.must(err)
					raw = encoded
				}
				links[r.ID] = append(links[r.ID], raw)
			}
			continue
		}
		for _, raw := range deps {
			var d kernel.Dependency
			e.must(json.Unmarshal(raw, &d))
			if !owned[d.ID] {
				links[r.ID] = append(links[r.ID], raw)
			}
		}
	}
	tail := []string{}
	specBranch, err := workspace.SpecBranch(e.f.SpecID)
	e.must(err)
	for _, line := range strings.Split(raw[dec.InputOffset():], "\n") {
		if line == "" {
			continue
		}
		allowed := strings.HasSuffix(line, "refs/heads/"+specBranch) || strings.HasSuffix(line, "refs/heads/main") || strings.HasSuffix(line, "refs/mindspec/merge-source/"+specBranch)
		for _, id := range e.f.Items {
			branch, err := workspace.BeadBranch(id)
			e.must(err)
			allowed = allowed || strings.HasSuffix(line, "refs/heads/"+branch) || strings.HasSuffix(line, "refs/mindspec/merge-source/"+branch)
		}
		if len(line) > 3 && strings.HasPrefix(line[3:], ".mindspec/specs/"+e.f.SpecID+"/reviews/") {
			allowed = true
		}
		if !allowed {
			tail = append(tail, line)
		}
	}
	data, err := json.Marshal(struct {
		Rows  []json.RawMessage
		Links map[string][]json.RawMessage
		Tail  []string
	}{kept, links, tail})
	e.must(err)
	return string(data)
}
func TestS134E2E_KernelPath(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t))
	e.apply()
	e.foreign()
	for i, item := range e.f.Items {
		if i == 1 {
			e.tracker("close", e.bug)
			e.apply()
		}
		out := e.implement(i)
		if len(out.Run) != 1 || !reflect.DeepEqual(out.Run[0].Argv, []string{"mindspec", "panel", "create", item}) {
			t.Fatalf("run after implementation: %+v", out)
		}
		e.create(out)
		slots := e.apply()
		n := 0
		for _, a := range slots.Actions {
			if strings.HasPrefix(a.Step, item+".review.r1.R") {
				n++
			}
		}
		if n != 3 {
			t.Fatalf("round slots: %+v", slots)
		}
		rounds, err := panel.RoundsForGate(panel.ForBead(panel.Scan(e.f.SpecDir), item), e.f.SpecID, "bead", item)
		e.must(err)
		if rounds != 1 {
			t.Fatalf("round count %d", rounds)
		}
		e.refuse("not authorized", "panel", "create", item)
		after, err := panel.RoundsForGate(panel.ForBead(panel.Scan(e.f.SpecDir), item), e.f.SpecID, "bead", item)
		e.must(err)
		if after != rounds {
			t.Fatalf("duplicate create changed round count: %d to %d", rounds, after)
		}
		e.review("APPROVE")
		out = e.apply()
		found := false
		for _, r := range out.Run {
			if reflect.DeepEqual(r.Argv, []string{"mindspec", "complete", item}) {
				e.ok(r.Argv[1:]...)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no completion: %+v", out)
		}
	}
	final := e.apply()
	if len(final.Run) != 1 || !reflect.DeepEqual(final.Run[0].Argv, []string{"mindspec", "panel", "create", e.f.SpecID}) {
		t.Fatalf("final review command: %+v", final)
	}
	e.create(final)
	e.review("APPROVE")
	out := e.apply()
	if len(out.Run) != 0 || out.Status != "wait" {
		t.Fatalf("approval offered over an open foreign child: %+v", out)
	}
	before := kerneltest.Snapshot(t, e.f)
	e.refuse(e.note, "impl", "approve", e.f.SpecID)
	if after := kerneltest.Snapshot(t, e.f); after != before {
		t.Fatal("foreign-child approval refusal wrote state")
	}
	e.tracker("close", e.note)
	out = e.apply()
	if len(out.Run) != 1 || !reflect.DeepEqual(out.Run[0].Argv, []string{"mindspec", "impl", "approve", e.f.SpecID}) {
		t.Fatalf("no approval: %+v", out)
	}
	e.ok(out.Run[0].Argv[1:]...)
	export, err := os.ReadFile(filepath.Join(e.f.Root, ".beads/issues.jsonl"))
	e.must(err)
	if !bytes.Contains(export, []byte(e.f.Epic)) {
		t.Fatal("finalize export missing")
	}
	if out = e.apply(); out.Status != "done" {
		t.Fatalf("not done: %+v", out)
	}
}
func TestS134E2E_PinnedPolicyAndRoundLimit(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	e.apply()
	cfg, err := os.ReadFile(filepath.Join(e.f.Root, ".mindspec/config.yaml"))
	e.must(err)
	e.commit(e.f.Root, ".mindspec/config.yaml", string(cfg)+"  max_rounds: 99\n  lease_ttl: 10m\n")
	e.create(e.implement(0))
	slots := e.apply()
	var first kernel.Action
	for _, a := range slots.Actions {
		if a.Kind == "review-slot" {
			first = a
			break
		}
	}
	if first.Step == "" {
		t.Fatal("no slot")
	}
	before := time.Now().UTC()
	e.lease(first.Step)
	after := time.Now().UTC()
	lease, ok, err := kernel.Decode[kernel.LeaseRecord](e.row(first.Step).Metadata, kernel.KeyLease)
	e.must(err)
	if !ok || lease.Expires.Before(before.Add(2*time.Hour)) || lease.Expires.After(after.Add(2*time.Hour)) {
		t.Fatalf("lease not pinned: %+v; issue interval %v..%v", lease, before, after)
	}
	for n := 2; n <= 5; n++ {
		e.commit(e.tree(e.f.Items[0]), fmt.Sprintf("round-%d.txt", n), "new subject\n")
		e.create(e.apply())
	}
	e.commit(e.tree(e.f.Items[0]), "round-6.txt", "new subject\n")
	out := e.apply()
	if out.Policy.RoundLimit != 5 || out.Policy.LeaseTTL != "2h" || len(out.Decisions) != 1 || len(out.Run) != 0 {
		t.Fatalf("unpinned limit: %+v", out)
	}
	e.refuse("limit", "panel", "create", e.f.Items[0])
	rounds, err := panel.RoundsForGate(panel.Scan(e.f.SpecDir), e.f.SpecID, "bead", e.f.Items[0])
	e.must(err)
	if rounds != 5 {
		t.Fatalf("sixth round registered: %d", rounds)
	}
}
func TestS134E2E_RejectRequiresSubmittedFix(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	e.apply()
	e.create(e.implement(0))
	e.review("REJECT")
	out := e.apply()
	if len(out.Decisions) != 1 {
		t.Fatalf("no reject decision: %+v", out)
	}
	e.ok("submit", out.Decisions[0].Step, "--answer", "fix")
	e.refuse("not authorized", "panel", "create", e.f.Items[0])
	fix := e.f.Items[0] + ".review.r2.fix"
	token := e.lease(fix)
	head := e.commit(e.tree(e.f.Items[0]), "fixed.txt", "fixed\n")
	e.refuse("not authorized", "panel", "create", e.f.Items[0])
	e.ok("submit", fix, "--lease", token, "--commit", head)
	e.create(e.apply())
	rounds, err := panel.RoundsForGate(panel.Scan(e.f.SpecDir), e.f.SpecID, "bead", e.f.Items[0])
	e.must(err)
	if rounds != 2 {
		t.Fatalf("fix did not authorize round two: %d", rounds)
	}
}
func TestS134E2E_ChangeJudgeVerbs(t *testing.T) {
	t.Parallel()
	bin := kernelE2EBinary(t)
	for _, mode := range []string{"carry-over", "re-panel", "rebind"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			e := newKernelE2E(t, bin, 2)
			e.apply()
			if mode == "rebind" {
				e.foreign()
				e.tracker("close", e.bug)
			}
			e.create(e.implement(0))
			e.review("APPROVE")
			item := e.f.Items[0]
			gate := item + ".review"
			path := ".mindspec/config.yaml"
			body := "ordinary change\n"
			if mode == "rebind" {
				path = "ordinary.txt"
			} else {
				data, err := os.ReadFile(filepath.Join(e.tree(item), path))
				e.must(err)
				body = string(data) + "  lease_ttl: 3h\n"
			}
			e.commit(e.tree(item), path, body)
			e.apply()
			judge := gate + ".judge.a1"
			if e.row(judge).Status != "open" {
				t.Fatal("judge absent")
			}
			e.refuse(judge, "complete", item)
			e.refuse(judge, "impl", "approve", e.f.SpecID)
			if mode == "rebind" {
				e.ok("panel", "rebind", item, "--spec", e.f.SpecID, "--reason", "The review covers this file.")
				r := e.row(judge)
				receipt, ok, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
				e.must(err)
				if r.Status != "closed" || !ok || receipt.Kind != "superseded" || !strings.Contains(receipt.Rebind, "rebind") {
					t.Fatalf("rebind receipt: %+v", r)
				}
			} else {
				e.judge(mode, 3)
				if mode == "re-panel" {
					e.create(e.apply())
					e.review("APPROVE")
				}
			}
			e.ok("complete", item)
			if e.row(e.f.Items[1]+".impl.a1").Status != "open" {
				t.Fatal("dependent not released")
			}
			out := e.apply()
			for _, a := range out.Actions {
				if a.Kind == "change-judge" {
					t.Fatalf("judge after complete: %+v", out)
				}
			}
			raw, code := e.run("impl", "approve", e.f.SpecID)
			// The audit can name a historical re-panel's successor gate. Only
			// the refusal describes records that still block approval.
			_, refusal, hasRefusal := strings.Cut(raw, "\nerror: ")
			if code == 0 || !hasRefusal || strings.Contains(refusal, gate) || strings.Contains(refusal, judge) {
				t.Fatalf("approval retained old gate/judge: %d %s", code, raw)
			}
			out = e.apply()
			for _, a := range out.Actions {
				if a.Kind == "change-judge" {
					t.Fatalf("judge after approval: %+v", out)
				}
			}
		})
	}
}
func (e *kernelE2E) mixedReview(words []string, repair bool) {
	e.t.Helper()
	out := e.apply()
	i := 0
	for _, a := range out.Actions {
		if a.Kind != "review-slot" {
			continue
		}
		if i >= len(words) {
			e.t.Fatalf("unexpected slot: %+v", a)
		}
		token := e.lease(a.Step)
		path := e.verdict(a, words[i])
		if repair && i == 0 {
			e.tracker("close", kerneltest.ID(e.t, e.f, a.Step))
		}
		r := e.row(a.Step)
		tuple, _, err := r.Tuple()
		e.must(err)
		reg, ok, err := kernel.Decode[kernel.RegistrationRecord](e.row(r.Parent).Metadata, kernel.KeyRegistration)
		e.must(err)
		if !ok {
			e.t.Fatal("no registration")
		}
		if i == 1 {
			e.ok("panel", "submit", reg.Slug, "--spec", e.f.SpecID, "--slot", fmt.Sprintf("R%d", tuple.Slot), "--round", fmt.Sprint(tuple.Round), "--lease", token, "--file", path)
		} else {
			e.ok("submit", a.Step, "--lease", token, "--artifact", path)
		}
		if repair && i == 0 {
			receipt, ok, err := kernel.Decode[kernel.Receipt](e.row(a.Step).Metadata, kernel.KeyClose)
			e.must(err)
			if !ok || !receipt.Repaired {
				e.t.Fatal("repair not recorded")
			}
			e.apply()
			retry, err := e.f.Store.Lookup(a.Step + ".a2")
			e.must(err)
			if retry != nil {
				e.t.Fatalf("repair created retry: %+v", retry)
			}
		}
		i++
	}
	if i != len(words) {
		e.t.Fatalf("slot count %d, want %d", i, len(words))
	}
}
func TestS134E2E_AuditProducersAndReplay(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 2)
	e.apply()
	e.foreign()
	item := e.f.Items[0]
	step := item + ".impl.a1"
	raw := e.ok("next", "1", "--allow-not-ready=MF-1", "--evidence", filepath.Join(e.f.SpecDir, "spec.md"))
	if !strings.Contains(raw, "generation 1") {
		t.Fatalf("next did not lease: %s", raw)
	}
	token := e.lease(step, "--take", "--reason", "Replace the implementation actor.")
	head := e.commit(e.tree(item), "implementation.txt", "implemented\n")
	e.ok("submit", step, "--lease", token, "--commit", head)
	e.create(e.apply())
	e.mixedReview([]string{"REJECT", "APPROVE", "APPROVE"}, false)
	out := e.apply()
	if len(out.Decisions) != 1 {
		t.Fatalf("no decision: %+v", out)
	}
	var ready []kernel.Record
	e.must(json.Unmarshal(e.tracker("ready", "--json", "--limit", "0"), &ready))
	found := false
	decisionID := kerneltest.ID(t, e.f, out.Decisions[0].Step)
	for _, r := range ready {
		if r.ID == decisionID {
			found = true
			if !strings.HasPrefix(r.Title, "[mindspec]") {
				t.Fatalf("unmarked decision: %+v", r)
			}
		}
	}
	if !found {
		t.Fatal("decision missing from ready")
	}
	e.ok("submit", out.Decisions[0].Step, "--answer", "fix")
	fix := item + ".review.r2.fix"
	token = e.lease(fix)
	head = e.commit(e.tree(item), "fixed.txt", "fixed\n")
	e.ok("submit", fix, "--lease", token, "--commit", head)
	e.create(e.apply())
	e.mixedReview([]string{"APPROVE", "APPROVE", "APPROVE"}, true)
	cfg, err := os.ReadFile(filepath.Join(e.tree(item), ".mindspec/config.yaml"))
	e.must(err)
	e.commit(e.tree(item), ".mindspec/config.yaml", string(cfg)+"  lease_ttl: 3h\n")
	e.judge("carry-over", 0)
	e.commit(e.tree(item), ".mindspec/config.yaml", string(cfg)+"  lease_ttl: 4h\n")
	e.judge("re-panel", 3)
	e.create(e.apply())
	e.mixedReview([]string{"REQUEST_CHANGES", "REQUEST_CHANGES", "APPROVE"}, false)
	var follow kernel.Record
	e.must(json.Unmarshal(e.tracker("create", "--title", "follow up", "--description", "fix the property", "--type", "task", "--json"), &follow))
	e.tracker("dep", "add", follow.ID, e.f.Epic, "--type", "discovered-from")
	e.ok("panel", "defer", item, "--spec", e.f.SpecID, "--slot", "R1", "--round", "3", "--by", "operator", "--bead", follow.ID)
	e.ok("panel", "refute", item, "--spec", e.f.SpecID, "--slot", "R2", "--round", "3", "--by", "operator", "--ground", "disproven", "--reason", "The implementation covers the finding.", "--evidence", head)
	e.ok("complete", item, "--allow-net-deletion", "The implementation is retained.")
	e.ok("complete", item, "--override-adr", "The accepted decision still applies.")
	e.ok("complete", item, "--allow-doc-skew", "The documentation is current.", "--doc-owner", e.f.Items[1])
	e.tracker("close", e.bug)
	e.apply()
	e.create(e.implement(1))
	e.review("APPROVE")
	e.ok("complete", e.f.Items[1])
	e.create(e.apply())
	e.review("APPROVE")
	e.refuse(e.note, "impl", "approve", e.f.SpecID)
	e.tracker("close", e.note)
	for i := 0; i < 2; i++ {
		audit := e.ok("impl", "approve", e.f.SpecID)
		for _, event := range []string{"--allow-not-ready", "--allow-doc-skew", "--override-adr", "--allow-net-deletion", "takeovers", "deferrals", "decision-answers", "unreceipted-closes", "carry-overs", "re-panels"} {
			if !auditHasCount(audit, event, "1") {
				t.Fatalf("audit %d missing one %s:\n%s", i, event, audit)
			}
		}
		for _, line := range strings.Split(audit, "\n") {
			if strings.HasPrefix(line, "re-panels |") && !strings.Contains(line, "3 slots") {
				t.Fatalf("re-panel slot count missing: %s", line)
			}
		}
	}
	for _, verb := range []string{"next-action", "lease", "submit", "next", "panel create", "panel submit", "panel refute", "panel defer", "complete", "impl approve"} {
		if !e.seen[verb] {
			t.Errorf("missing snapshot for %s", verb)
		}
	}
}
func TestS134E2E_SharedReadyViews(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	e.apply()
	e.foreign()
	var child kernel.Record
	e.must(json.Unmarshal(e.tracker("create", "--parent", e.f.Epic, "--title", "unlabelled foreign child", "--no-inherit-labels", "--json"), &child))
	e.unlabelled = child.ID
	e.ready()
}
func TestS134E2E_RunPanelTargetsUseSpecWorktree(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	name, err := workspace.SpecWorktreeName(f.SpecID)
	f.Must(err)
	dir := filepath.Join(f.Root, ".worktrees", name, ".mindspec/specs", f.SpecID)
	f.Must(os.MkdirAll(filepath.Dir(dir), 0700))
	f.Must(os.Rename(f.SpecDir, dir))
	f.SpecDir = dir
	if got := panelCreateKernelGate(f.Root, f.Items[0], "", "", ""); got != "bead" {
		t.Fatalf("item target resolved as %q", got)
	}
	if got := panelCreateKernelGate(f.Root, f.SpecID, "", "", ""); got != "final_review" {
		t.Fatalf("final target resolved as %q", got)
	}
	out, err := kernelPanelRun(t, panelCreateCmd, f.Items[0])
	kernelPanelRequire(t, out, err)
	registration, ok, err := kernel.Decode[kernel.RegistrationRecord](kernelPanelRecord(t, f, gate).Metadata, kernel.KeyRegistration)
	f.Must(err)
	rounds, err := panel.RoundsForGate(panel.Scan(dir), f.SpecID, "bead", f.Items[0])
	f.Must(err)
	if !ok || registration.Round != 1 || registration.Slug != f.Items[0] || rounds != 1 {
		t.Fatalf("returned command did not register round one: %+v, rounds %d", registration, rounds)
	}
}

func TestS134PanelCreate_LegacySpecShorthand(t *testing.T) {
	f, _, _ := kernelPanelFixture(t)
	f.Must(os.MkdirAll(filepath.Join(filepath.Dir(f.SpecDir), "002-legacy"), 0700))
	if got := panelCreateKernelGate(f.Root, "002-legacy", "", "", ""); got != "" {
		t.Fatalf("legacy spec shorthand claimed as %q", got)
	}
	before := kerneltest.Snapshot(t, f)
	out, err := kernelPanelRun(t, panelCreateCmd, "002-legacy")
	kernelPanelRefusal(t, out, err, "--spec is required")
	if after := kerneltest.Snapshot(t, f); after != before {
		t.Fatal("legacy panel command changed enrolled state")
	}
}

func auditHasCount(audit, event, count string) bool {
	for _, line := range strings.Split(audit, "\n") {
		fields := strings.Split(line, " | ")
		if len(fields) >= 2 && fields[0] == event && fields[1] == count {
			return true
		}
	}
	return false
}

func TestS134Audit_ExactCount(t *testing.T) {
	for _, count := range []string{"0", "1", "2", "10", "11", "19"} {
		for _, suffix := range []string{"", " | 3 slots"} {
			if got := auditHasCount("other | 1\nre-panels | "+count+suffix+"\n", "re-panels", "1"); got != (count == "1") {
				t.Fatalf("count %s%s accepted=%t", count, suffix, got)
			}
		}
	}
}

func TestS134E2E_ProtectedSnapshotComputedRefs(t *testing.T) {
	t.Parallel()
	f := kerneltest.Enrolled(t, kerneltest.WithFake(kernel.NativeIDs))
	f.Fake.Records["foreign-note"] = kernel.Record{ID: "foreign-note", Title: "foreign", Type: "task", Status: "open", Parent: f.Epic}
	e := &kernelE2E{t: t, f: f}
	before := e.protectedSnapshot()
	branch, err := workspace.BeadBranch(f.Items[0])
	e.must(err)
	head := e.git(f.Root, "rev-parse", "HEAD")
	e.git(f.Root, "update-ref", "refs/mindspec/merge-source/"+branch, head)
	if after := e.protectedSnapshot(); after != before {
		t.Fatal("computed merge marker changed protected snapshot")
	}
	foreign, err := workspace.BeadBranch("foreign-note")
	e.must(err)
	e.git(f.Root, "update-ref", "refs/mindspec/merge-source/"+foreign, head)
	if after := e.protectedSnapshot(); after == before {
		t.Fatal("foreign merge marker escaped snapshot detector")
	}
}

// Use the command entry point with a subprocess fake tracker. This isolates the
// publication wiring from the longer real-store lifecycle and runs in short mode.
func TestS134SubmitCmd_PublishesCommittedReview(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	id := gate + ".r1.R1"
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	v := paneltest.Verdict("APPROVE")
	v.Slot, v.Round, v.ReviewedSHA, v.Model = "R1", 1, head, "wrong-model"
	file := filepath.Join(f.SpecDir, "reviews", "1", "committed-verdict.json")
	data := paneltest.Bytes(t, v)
	f.Must(os.WriteFile(file, data, 0600))
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "verdict")
	store := actorCmdStore(t, f)
	out, err = kernelPanelRun(t, submitCmd, id, "--lease", "wrong-token", "--artifact", file)
	kernelPanelRefusal(t, out, err, "lease token is not current")
	subs, err := panel.LatestSubmissions(filepath.Dir(file), 1)
	f.Must(err)
	if len(subs) != 0 {
		t.Fatal("invalid submission published a verdict", subs)
	}
	ledgerBefore, err := panel.ReadLedger(filepath.Dir(file))
	f.Must(err)
	out, err = kernelPanelRun(t, submitCmd, id, "--lease", lease.Token, "--artifact", file)
	kernelPanelRefusal(t, out, err, "effective model")
	row := actorCmdRead(t, store, id)
	_, hasReceipt, err := kernel.Decode[kernel.Receipt](row.Metadata, kernel.KeyClose)
	f.Must(err)
	ledgerAfter, err := panel.ReadLedger(filepath.Dir(file))
	f.Must(err)
	subs, err = panel.LatestSubmissions(filepath.Dir(file), 1)
	f.Must(err)
	if row.Status != "in_progress" || hasReceipt || len(subs) != 0 || !reflect.DeepEqual(ledgerBefore, ledgerAfter) {
		t.Fatal("refused publication closed the slot, wrote a receipt, or changed the ledger", row, subs)
	}
	v.Model = "test-model"
	f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0600))
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "correct verdict")
	out, err = kernelPanelRun(t, submitCmd, id, "--lease", lease.Token, "--artifact", file)
	kernelPanelRequire(t, out, err)
	subs, err = panel.LatestSubmissions(filepath.Dir(file), 1)
	f.Must(err)
	row = actorCmdRead(t, store, id)
	if row.Status != "closed" || len(subs) != 1 || subs["R1"].Verdict != "APPROVE" || subs["R1"].Seq != 1 {
		t.Fatal("generic submit did not publish its committed verdict", row, subs)
	}
	receipt, ok, err := kernel.Decode[kernel.Receipt](row.Metadata, kernel.KeyClose)
	f.Must(err)
	if !ok {
		t.Fatal("published submission has no receipt")
	}
	f.Must(kernel.ValidReceipt(f.Root, receipt))
	ledgerBefore, err = panel.ReadLedger(filepath.Dir(file))
	f.Must(err)
	before := kerneltest.Snapshot(t, f)
	out, err = kernelPanelRun(t, submitCmd, id, "--lease", lease.Token, "--artifact", file)
	kernelPanelRequire(t, out, err)
	ledgerAfter, err = panel.ReadLedger(filepath.Dir(file))
	f.Must(err)
	if !reflect.DeepEqual(ledgerBefore, ledgerAfter) || before != kerneltest.Snapshot(t, f) {
		t.Fatal("identical resubmit changed the ledger or tracker")
	}
}

func TestS134SubmitCmd_RefreshesPublicationState(t *testing.T) {
	f, gate, head := kernelPanelFixture(t)
	kernelPanelAuthorise(f, gate, head, 1, 2)
	stale := f.State()
	if len(stale.Panels) != 0 {
		t.Fatal("stale fixture already has a panel")
	}
	out, err := kernelPanelCreate(t, f, "1")
	kernelPanelRequire(t, out, err)
	id := gate + ".r1.R1"
	lease, err := kernel.Lease(f.State(), kernel.LeaseOptions{Step: id}, kernel.LeaseDeps{})
	f.Must(err)
	v := paneltest.Verdict("APPROVE")
	v.Slot, v.Round, v.ReviewedSHA, v.Model = "R1", 1, head, "test-model"
	file := filepath.Join(f.SpecDir, "reviews", "1", "committed-verdict.json")
	f.Must(os.WriteFile(file, paneltest.Bytes(t, v), 0600))
	actorCmdGit(t, f, "add", ".")
	actorCmdGit(t, f, "-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "commit", "-qm", "verdict")
	// Keep the command's initial read before registration; the actor lock must refresh it.
	old := submitCommandStateFn
	submitCommandStateFn = func(_, target string) (kernel.State, error) {
		if target != id {
			t.Fatalf("unexpected target %s", target)
		}
		return stale, nil
	}
	t.Cleanup(func() { submitCommandStateFn = old })
	out, err = kernelPanelRun(t, submitCmd, id, "--lease", lease.Token, "--artifact", file)
	kernelPanelRequire(t, out, err)
	subs, err := panel.LatestSubmissions(filepath.Dir(file), 1)
	f.Must(err)
	row := kernelPanelRecord(t, f, id)
	receipt, ok, err := kernel.Decode[kernel.Receipt](row.Metadata, kernel.KeyClose)
	f.Must(err)
	if row.Status != "closed" || !ok || len(subs) != 1 || subs["R1"].Verdict != "APPROVE" || subs["R1"].Seq != 1 {
		t.Fatal("submit did not publish with refreshed registration", row.Status, ok, subs)
	}
	f.Must(kernel.ValidReceipt(f.Root, receipt))
}

func TestS134PanelCreate_NonEnrolledSpecShorthand(t *testing.T) {
	f, _, _ := kernelPanelFixture(t)
	state := f.State()
	state.Enrolled = false
	panelKernelStateFn = func(_, _ string) (*kernel.State, error) { return &state, nil }
	before := kerneltest.Snapshot(t, f)
	if got := panelCreateKernelGate(f.Root, f.SpecID, "", "", ""); got != "" {
		t.Fatalf("non-enrolled spec shorthand claimed as %q", got)
	}
	if after := kerneltest.Snapshot(t, f); after != before {
		t.Fatal("shorthand lookup changed state")
	}
}

func TestS134PanelSubmit_AttemptDirectoryBoundary(t *testing.T) {
	t.Run("stray-slot", func(t *testing.T) {
		f := newCLIV2(t)
		f.shortSetup(t, 1)
		parent := filepath.Join(f.layout.Tree, "nested")
		nested := filepath.Join(parent, "fixture")
		sub := filepath.Join(nested, "sub")
		if err := os.MkdirAll(sub, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(nested, ".mindspec"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(parent, "slot.json"), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, cwd := range []string{nested, sub} {
			slot, found, err := findPanelSubmitSlot(cwd)
			if err != nil || found {
				t.Errorf("nested workspace captured: %+v %t %v", slot, found, err)
			}
		}
	})
	for _, marker := range []string{".git", ".mindspec"} {
		t.Run(marker, func(t *testing.T) {
			f := newCLIV2(t)
			file := f.shortSetup(t, 1)
			attempt := filepath.Dir(file)
			sub := filepath.Join(attempt, "sub")
			if err := os.Mkdir(sub, 0700); err != nil {
				t.Fatal(err)
			}
			for _, dir := range []string{attempt, sub} {
				slot, found, err := findPanelSubmitSlot(dir)
				if err != nil || !found || slot.Dir != f.layout.Dir {
					t.Fatalf("attempt lookup: %+v %t %v", slot, found, err)
				}
			}
			for _, repo := range []string{sub, attempt} {
				if err := os.Mkdir(filepath.Join(repo, marker), 0700); err != nil {
					t.Fatal(err)
				}
				for _, dir := range []string{repo, sub} {
					slot, found, err := findPanelSubmitSlot(dir)
					if err != nil || found {
						t.Fatalf("attempt workspace captured: %+v %t %v", slot, found, err)
					}
				}
			}
		})
	}
	for _, name := range []string{"attempt-01", "attempt-+1"} {
		t.Run(name, func(t *testing.T) {
			f := newCLIV2(t)
			f.shortSetup(t, 1)
			dir := filepath.Join(f.layout.Dir, name, "sub")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			slot, found, err := findPanelSubmitSlot(dir)
			if err != nil || found {
				t.Fatalf("noncanonical attempt accepted: %+v %t %v", slot, found, err)
			}
		})
	}
}
