package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
)

// preKernelLoop runs a spec that is not enrolled with only the commands that
// the binary prints: refusals, recovery lines, instruct and the panel tally
// Next block. A command runs only when a printed line names its verb and each
// of its flags.
type preKernelLoop struct {
	t       *testing.T
	f       *kerneltest.Fixture
	bin     string
	printed []string
}

var preKernelMention = regexp.MustCompile("mindspec [a-z][^`\n]*")

func (p *preKernelLoop) git(dir string, args ...string) string {
	p.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Kernel Test", "-c", "user.email=kernel@example.invalid", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		p.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (p *preKernelLoop) write(path, body string) {
	p.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		p.t.Fatal(err)
	}
}

// run executes the binary and records every command line its output names.
func (p *preKernelLoop) run(dir string, args ...string) (string, bool) {
	p.t.Helper()
	out, err := p.f.BinaryCommandIn(dir, p.bin, args...)
	p.t.Logf("binary %v (err %v):\n%s", args, err, out)
	for _, m := range preKernelMention.FindAllString(string(out), -1) {
		for _, part := range strings.Split(m, ", mindspec ") {
			if !strings.HasPrefix(part, "mindspec ") {
				part = "mindspec " + part
			}
			p.printed = append(p.printed, part)
		}
	}
	return string(out), err == nil
}

// allowed reports whether a printed line names the verb and each flag of args.
func (p *preKernelLoop) allowed(verb string, args []string) bool {
	for _, line := range p.printed {
		rest, ok := strings.CutPrefix(line, "mindspec "+verb)
		if !ok || (rest != "" && !strings.ContainsAny(rest[:1], " ,'\"")) {
			continue
		}
		all := true
		for _, arg := range args {
			if strings.HasPrefix(arg, "--") && !regexp.MustCompile(`(^|\s)`+regexp.QuoteMeta(arg)+`(\s|=|$)`).MatchString(rest) {
				all = false
			}
		}
		if all {
			return true
		}
	}
	return false
}

// try runs a printed command and returns its output and success.
func (p *preKernelLoop) try(dir, verb string, args ...string) (string, bool) {
	p.t.Helper()
	if !p.allowed(verb, args) {
		p.t.Fatalf("mindspec %s %v was not printed; printed lines:\n%s", verb, args, strings.Join(p.printed, "\n"))
	}
	return p.run(dir, append(strings.Fields(verb), args...)...)
}

// do runs a printed command and requires exit 0.
func (p *preKernelLoop) do(dir, verb string, args ...string) string {
	p.t.Helper()
	out, ok := p.try(dir, verb, args...)
	if !ok {
		p.t.Fatalf("mindspec %s %v failed:\n%s", verb, args, out)
	}
	return out
}

// preKernelLine returns the first line of out that contains command, from the
// command on.
func preKernelLine(t *testing.T, out, command string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, command); i >= 0 {
			return strings.TrimSpace(line[i:])
		}
	}
	t.Fatalf("output does not print %q:\n%s", command, out)
	return ""
}

// gate creates a panel from the create line that refusal prints, reviews each
// slot that the tally Next block names, and requires that the passing tally
// prints the verb next.
func (p *preKernelLoop) gate(dir, refusal, slug, verb string) {
	p.t.Helper()
	create := preKernelLine(p.t, refusal, "mindspec panel create <slug>")
	fields := strings.Fields(strings.TrimPrefix(create, "mindspec panel create <slug>"))
	p.do(dir, "panel create", append([]string{slug}, fields...)...)
	pass := p.review(dir, slug)
	if !regexp.MustCompile(`Next:\n  mindspec ` + regexp.QuoteMeta(verb) + `\n`).MatchString(strings.ReplaceAll(pass, "'", "")) {
		p.t.Fatalf("passing tally does not print mindspec %s next:\n%s", verb, pass)
	}
}

// review reviews each slot that the tally Next block names, with only the
// commands that the tally and the slot prompts print, and returns the passing
// tally output.
func (p *preKernelLoop) review(dir, slug string) string {
	p.t.Helper()
	tally, ok := p.try(dir, "panel tally", slug)
	if ok || !strings.Contains(tally, "Next:") {
		p.t.Fatalf("incomplete tally printed no Next block:\n%s", tally)
	}
	slots := regexp.MustCompile(`mindspec panel checkout '[^']+' --slot '(R\d+)' --dispatch`).FindAllStringSubmatch(tally, -1)
	if len(slots) == 0 {
		p.t.Fatalf("no slot to review:\n%s", tally)
	}
	for _, m := range slots {
		slot := m[1]
		out, ok := p.try(dir, "panel checkout", slug, "--slot", slot, "--dispatch")
		if !ok {
			// The refusal names the brief file to fill.
			m := regexp.MustCompile(`panel brief (\S+) contains TODO\(skill\) stubs; fill it`).FindStringSubmatch(out)
			if m == nil {
				p.t.Fatalf("checkout refused:\n%s", out)
			}
			data, err := os.ReadFile(m[1])
			if err != nil {
				p.t.Fatal(err)
			}
			p.write(m[1], regexp.MustCompile(`<!-- TODO\(skill\): ([^>]*) -->`).ReplaceAllString(string(data), "Filled: $1."))
			p.do(dir, "panel checkout", slug, "--slot", slot, "--dispatch")
		}
		prompt := p.do(dir, "panel prompt", slug, "--slot", slot)
		sha := regexp.MustCompile("Reviewed commit\\*\\*: `([0-9a-f]{40})`").FindStringSubmatch(prompt)
		round := regexp.MustCompile(`(?m)^- \*\*Round\*\*: (\d+)$`).FindStringSubmatch(prompt)
		file := regexp.MustCompile(`mindspec panel submit --file '([^']+)'`).FindStringSubmatch(prompt)
		if sha == nil || round == nil || file == nil {
			p.t.Fatalf("slot prompt lacks the commit, round or submission command:\n%s", prompt)
		}
		v := paneltest.Verdict("APPROVE")
		v.Slot = slot
		fmt.Sscan(round[1], &v.Round)
		v.ReviewedSHA = sha[1]
		v.Model = "test-model"
		data, err := json.Marshal(v)
		if err != nil {
			p.t.Fatal(err)
		}
		p.write(file[1], string(data))
		p.do(dir, "panel submit", "--file", file[1])
	}
	return p.do(dir, "panel tally", slug)
}

// approve runs a printed approval verb, which refuses with no panel; it then
// passes the gate and requires the verb to exit 0.
func (p *preKernelLoop) approve(dir, verb, slug string, args ...string) string {
	p.t.Helper()
	out, ok := p.try(dir, verb, args...)
	if ok {
		p.t.Fatalf("mindspec %s passed with no panel:\n%s", verb, out)
	}
	p.gate(dir, out, slug, verb+" "+args[0])
	return p.do(dir, verb, args...)
}

func TestS134E2E_PreKernelPrintedCommands(t *testing.T) {
	t.Parallel()
	bin := kernelE2EBinary(t)
	f := kerneltest.PreKernelSpec(t)
	p := &preKernelLoop{t: t, f: f, bin: bin}
	root := f.Root
	p.write(filepath.Join(root, ".mindspec/config.yaml"), "enforcement:\n  panel_gate: true\n  panel_presence: enforce\nauto_open_finalize_pr: false\npanel:\n  reviewers:\n    - family: harness\n      model: test-model\n      lens: correctness\n      count: 2\n")
	p.write(filepath.Join(root, ".gitignore"), ".worktrees/\n.mindspec-scratch\n.mindspec/phase.lock\n.beads/*\n!.beads/config.yaml\n")
	p.write(filepath.Join(root, ".mindspec/adr/ADR-9001.md"), "# ADR-9001: Fixture\n\n- **Status**: Accepted\n- **Domain(s)**: workflow\n\n## Decision\nExercise the lifecycle.\n")
	plan := strings.Replace(kerneltest.EnrolmentPlan(f.SpecID, 3), "status: Draft\n", "status: Approved\nadr_citations: [ADR-9001]\nbead_ids:\n  - "+strings.Join(f.Items, "\n  - ")+"\n", 1)
	p.write(filepath.Join(f.SpecDir, "spec.md"), "---\nstatus: Approved\n---\n# Spec: 002-y\n\n## Goal\nExercise the lifecycle.\n\n## Impacted Domains\n\n- workflow\n\n## ADR Touchpoints\n\n- ADR-9001\n\n## Requirements\n\n1. Requirement one\n2. Requirement two\n\n## Scope\n\n### In Scope\n\n- N/A\n\n### Out of Scope\n\n- N/A\n\n## Acceptance Criteria\n\n- [ ] Criterion one\n- [ ] Criterion two\n- [ ] Criterion three\n\n## Open Questions\n\nNone.\n")
	p.write(filepath.Join(f.SpecDir, "plan.md"), readyPlan(plan))
	if out, ok := p.run(root, "setup"); !ok {
		t.Fatalf("setup:\n%s", out)
	}
	p.git(root, "add", "-A")
	p.git(root, "commit", "-qm", "fixture")
	p.git(root, "branch", "-M", "main")
	p.git(root, "branch", "spec/"+f.SpecID)
	cwd := filepath.Join(root, ".worktrees", "worktree-spec-"+f.SpecID)
	p.git(root, "worktree", "add", cwd, "spec/"+f.SpecID)

	refusal, ok := p.run(cwd, "next-action", f.SpecID, "--apply")
	if ok || !strings.Contains(refusal, "recovery: mindspec next") {
		t.Fatalf("next-action did not refuse with the existing cycle:\n%s", refusal)
	}
	if out, ok := p.run(cwd, "instruct"); !ok || !strings.Contains(out, "mindspec impl approve") {
		t.Fatalf("instruct does not print the lifecycle:\n%s", out)
	}
	p.approve(cwd, "spec approve", "spec-gate", f.SpecID)
	out := p.approve(cwd, "plan approve", "plan-gate", f.SpecID)
	if !strings.Contains(out, "Run `mindspec next`") {
		t.Fatalf("plan approve did not keep the existing lifecycle:\n%s", out)
	}
	if refusal, ok := p.run(cwd, "next-action", f.SpecID, "--apply"); ok || !strings.Contains(refusal, "recovery: mindspec next") {
		t.Fatalf("plan approve enrolled the spec:\n%s", refusal)
	}
	for i, item := range f.Items {
		out := p.do(cwd, "next")
		m := regexp.MustCompile(`(?m)^Worktree: (.+)$`).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("next printed no worktree:\n%s", out)
		}
		bt := m[1]
		p.write(filepath.Join(bt, fmt.Sprintf("implement-%d.txt", i+1)), "work\n")
		p.git(bt, "add", "-A")
		p.git(bt, "commit", "-qm", "work")
		p.approve(bt, "complete", fmt.Sprintf("b%d", i+1), item, "done")
	}
	out = p.approve(cwd, "impl approve", "final", f.SpecID)
	if !strings.Contains(out, "Implementation for "+f.SpecID+" approved.") {
		t.Fatalf("impl approve did not approve:\n%s", out)
	}
	records, err := f.Store.All()
	f.Must(err)
	for _, r := range records {
		if r.Type == "step" || r.Type == "decision" {
			t.Fatalf("the existing lifecycle wrote kernel record %s (%s)", r.ID, r.Type)
		}
	}
}

// adapterSection returns the text of one "## " section of an installed adapter.
func adapterSection(t *testing.T, root, adapter, heading string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".claude", "skills", adapter, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(data), "\n## "+heading+"\n")
	if !ok {
		t.Fatalf("%s has no %s section", adapter, heading)
	}
	section, _, _ = strings.Cut(section, "\n## ")
	return section
}

// adapterPanelCommands returns the commands, in order, that an installed
// adapter names for a run started for a panel.
func adapterPanelCommands(t *testing.T, root, adapter string) []string {
	t.Helper()
	for _, line := range strings.Split(adapterSection(t, root, adapter, "Panel"), "\n") {
		if strings.HasPrefix(line, "When the user starts this skill for a panel") {
			var out []string
			for _, m := range regexp.MustCompile("`(mindspec [^`]+)`").FindAllStringSubmatch(line, -1) {
				out = append(out, m[1])
			}
			return out
		}
	}
	t.Fatalf("%s names no command for a panel", adapter)
	return nil
}

// adapterTallyRoute is one bullet of the Panel section: the tally result it
// applies to and what the adapter does then.
type adapterTallyRoute struct {
	condition, action string
}

// adapterTallyRoutes returns every bullet of the Panel section. A bullet that
// this test cannot classify fails the test.
func adapterTallyRoutes(t *testing.T, root, adapter string) []adapterTallyRoute {
	t.Helper()
	var routes []adapterTallyRoute
	for _, line := range strings.Split(adapterSection(t, root, adapter, "Panel"), "\n") {
		rest, ok := strings.CutPrefix(line, "- It ")
		if !ok {
			continue
		}
		condition, action, ok := strings.Cut(rest, ": ")
		if !ok {
			t.Fatalf("%s: tally route %q has no action", adapter, line)
		}
		routes = append(routes, adapterTallyRoute{condition: condition, action: action})
	}
	if len(routes) == 0 {
		t.Fatalf("%s states no tally route", adapter)
	}
	return routes
}

// applies reports whether the route's condition holds for a tally output.
func (r adapterTallyRoute) applies(t *testing.T, out string, ok bool) bool {
	t.Helper()
	hasNext := regexp.MustCompile(`(?m)^Next:$`).MatchString(out)
	switch {
	case strings.HasPrefix(r.condition, "exits non-zero and prints no `decision:` line"):
		return !ok && !strings.Contains(out, "decision:")
	case strings.HasPrefix(r.condition, "prints a `Next` block"):
		return hasNext
	case strings.HasPrefix(r.condition, "prints no `Next` block"):
		return !hasNext
	}
	t.Fatalf("unknown tally route condition %q", r.condition)
	return false
}

// tallyNextBlock returns the lines of the Next block of a tally output.
func tallyNextBlock(out string) []string {
	_, block, ok := strings.Cut(out, "\nNext:\n")
	if !ok {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(block, "\n") {
		if !strings.HasPrefix(line, "  ") {
			break
		}
		lines = append(lines, strings.TrimPrefix(line, "  "))
	}
	return lines
}

func TestS134E2E_AdhocGateOnEnrolledSpec(t *testing.T) {
	t.Parallel()
	e := newKernelE2E(t, kernelE2EBinary(t), 1)
	item := e.f.Items[0]
	step := item + ".impl.a1"
	authorised := func(args ...string) {
		t.Helper()
		raw, code := e.run(append([]string{"next-action", e.f.SpecID, "--json"}, args...)...)
		var out kernel.Output
		if err := json.Unmarshal([]byte(strings.Split(raw, "\n")[0]), &out); err != nil {
			t.Fatalf("next-action %v: exit %d: %v\n%s", args, code, err, raw)
		}
		for _, a := range out.Actions {
			if a.Step == step {
				return
			}
		}
		t.Fatalf("next-action %v does not authorise the work item: %+v", args, out)
	}
	rows := func() string {
		t.Helper()
		records, err := e.f.Store.All()
		e.must(err)
		data, err := json.Marshal(records)
		e.must(err)
		return string(data)
	}
	p := &preKernelLoop{t: t, f: e.f, bin: e.bin}
	if out, ok := p.run(e.f.Root, "setup", "claude"); !ok {
		t.Fatalf("setup:\n%s", out)
	}
	authorised()
	before := rows()
	if strings.Contains(before, step) {
		t.Fatalf("the work item step exists before the adhoc run: %s", before)
	}
	for i, adapter := range []string{"ms-panel-tally", "ms-bead-cycle"} {
		slug := fmt.Sprintf("a%d", i+1)
		brief := filepath.Join(t.TempDir(), "brief.md")
		p.write(brief, "# Review brief\n\nReview the spec branch for correctness.\n")
		if out, ok := p.run(e.cwd, "panel", "create", slug, "--gate", "adhoc", "--target", "spec/"+e.f.SpecID, "--brief-file", brief); !ok {
			t.Fatalf("adhoc panel create:\n%s", out)
		}
		commands := adapterPanelCommands(t, e.f.Root, adapter)
		if len(commands) != 1 || !strings.HasPrefix(commands[0], "mindspec panel tally ") {
			t.Fatalf("%s names %q for a panel, want one panel tally", adapter, commands)
		}
		p.printed = append(p.printed, commands[0])
		pass := p.review(e.cwd, slug)
		if !strings.Contains(pass, "decision: PASS") {
			t.Fatalf("%s did not pass the adhoc panel:\n%s", adapter, pass)
		}
		// Follow every route of the adapter for this tally output.
		var taken []adapterTallyRoute
		for _, r := range adapterTallyRoutes(t, e.f.Root, adapter) {
			if r.applies(t, pass, true) {
				taken = append(taken, r)
			}
		}
		if len(taken) != 1 {
			t.Fatalf("%s: %d tally routes apply to a passing adhoc tally, want 1: %+v", adapter, len(taken), taken)
		}
		action := taken[0].action
		switch {
		case strings.Contains(action, "go to the loop"):
			loop := regexp.MustCompile("(?m)^1\\. Run `(mindspec [^`]+)`").FindStringSubmatch(adapterSection(t, e.f.Root, adapter, "Loop"))
			if loop == nil {
				t.Fatalf("%s: the loop has no first command", adapter)
			}
			argv := strings.Fields(strings.ReplaceAll(strings.TrimPrefix(loop[1], "mindspec "), "<spec-slug>", e.f.SpecID))
			p.run(e.cwd, argv...)
		case strings.Contains(action, "run the lines of that block"):
			for _, line := range tallyNextBlock(pass) {
				i := strings.Index(line, "mindspec ")
				if i < 0 || strings.Contains(line[:i], "operator") {
					continue
				}
				command := line[i:]
				if strings.Contains(command, "<") {
					t.Fatalf("%s: a passing adhoc Next block names a placeholder: %q", adapter, line)
				}
				p.run(e.cwd, strings.Fields(strings.ReplaceAll(strings.TrimPrefix(command, "mindspec "), "'", ""))...)
			}
		case strings.Contains(action, "report the error"):
		default:
			t.Fatalf("%s: unknown tally route action %q", adapter, action)
		}
		if after := rows(); after != before {
			t.Fatalf("%s: the adhoc panel wrote kernel state\nbefore: %s\nafter: %s", adapter, before, after)
		}
		if block := tallyNextBlock(pass); len(block) == 0 || strings.Contains(strings.Join(block, "\n"), "mindspec") {
			t.Fatalf("%s: a passing adhoc tally must print a Next block with no command:\n%s", adapter, pass)
		}
	}
	authorised("--apply")
}
