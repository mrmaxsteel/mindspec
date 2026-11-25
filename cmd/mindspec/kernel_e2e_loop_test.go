package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kerneltest"
	"github.com/mrmaxsteel/mindspec/internal/rules"
)

// readyPlan declares the files of each item, so the work-item brief that the
// implement action prints passes its readiness review.
func readyPlan(plan string) string {
	return regexp.MustCompile(`(?m)^(  - id: (\d+)\n    depends_on: .*)$`).ReplaceAllString(plan, "$1\n    key_file_paths: [implement-$2.txt]")
}

// printedLoop runs a spec with only the commands the binary prints: actions,
// decisions and run entries of next-action and submit, and the brief texts.
type printedLoop struct {
	*kernelE2E
	printed   map[string]bool
	kinds     map[string]int
	briefs    map[string]string
	briefArgv map[string]string
	commits   int
}

func newPrintedLoop(e *kernelE2E) *printedLoop {
	return &printedLoop{kernelE2E: e, printed: map[string]bool{}, kinds: map[string]int{}, briefs: map[string]string{}, briefArgv: map[string]string{}}
}

func (p *printedLoop) allow(out kernel.Output) {
	for _, a := range out.Actions {
		for _, argv := range [][]string{a.Claim, a.Brief, a.Submit} {
			p.printed[strings.Join(argv, "\x00")] = true
		}
	}
	for _, d := range out.Decisions {
		p.printed[strings.Join(d.Submit, "\x00")] = true
	}
	for _, r := range out.Run {
		p.printed[strings.Join(r.Argv, "\x00")] = true
	}
}

// do runs a printed template after it fills the named placeholders.
func (p *printedLoop) do(template []string, fill map[string]string) string {
	p.t.Helper()
	if len(template) < 2 || template[0] != "mindspec" || !p.printed[strings.Join(template, "\x00")] {
		p.t.Fatalf("command was not printed: %q", template)
	}
	argv := append([]string{}, template[1:]...)
	for i, arg := range argv {
		if strings.HasPrefix(arg, "<") && strings.HasSuffix(arg, ">") {
			value, ok := fill[arg]
			if !ok {
				p.t.Fatalf("no value for %s in %q", arg, template)
			}
			argv[i] = value
		}
	}
	return p.ok(argv...)
}

func (p *printedLoop) submit(a kernel.Action, fill map[string]string) {
	p.t.Helper()
	var out kernel.Output
	p.must(json.Unmarshal([]byte(p.do(a.Submit, fill)), &out))
	p.allow(out)
}

var printedRecordPath = regexp.MustCompile(`(?m)^Record path: (.+)$`)
var printedFinding = regexp.MustCompile(`(?m)^Finding: (R\d+) round (\d+) (\S+)\nDigest: (\S*)$`)

func (p *printedLoop) recordPath(brief string) string {
	p.t.Helper()
	m := printedRecordPath.FindStringSubmatch(brief)
	if m == nil {
		p.t.Fatalf("brief names no record path: %s", brief)
	}
	return m[1]
}

func (p *printedLoop) writeRecord(path string, v any) {
	p.t.Helper()
	data, err := json.Marshal(v)
	p.must(err)
	p.must(os.MkdirAll(filepath.Dir(path), 0700))
	p.must(os.WriteFile(path, data, 0600))
}

// act claims, reads the brief of, and returns one step.
func (p *printedLoop) act(a kernel.Action, verdict func(kernel.Action) string) {
	p.t.Helper()
	p.kinds[a.Kind]++
	claim := strings.Fields(p.do(a.Claim, nil))
	if len(claim) != 3 {
		p.t.Fatalf("lease output: %q", claim)
	}
	token := claim[2]
	brief := p.do(a.Brief, nil)
	p.briefs[a.Kind] = brief
	p.briefArgv[a.Kind] = strings.Join(a.Brief, " ")
	fill := map[string]string{"<token>": token}
	switch a.Kind {
	case "implement", "fix":
		tuple, _, err := p.row(a.Step).Tuple()
		p.must(err)
		root := p.cwd
		if tuple.WorkItem != "" {
			root = p.tree(tuple.WorkItem)
		}
		p.commits++
		fill["<commit>"] = p.commit(root, fmt.Sprintf("%s-%d.txt", a.Kind, p.commits), "work\n")
	case "review-slot":
		if !strings.Contains(brief, a.Subject.Commit) {
			p.t.Fatalf("slot brief lacks its subject %s: %s", a.Subject.Commit, brief)
		}
		fill["<path>"] = p.verdict(a, verdict(a))
	case "lens":
		path := p.recordPath(brief)
		record := kernel.LensRecord{Step: a.Step, Lenses: []kernel.SlotLens{}}
		for _, slot := range p.lensSlots(a) {
			record.Lenses = append(record.Lenses, kernel.SlotLens{Slot: slot, Lens: "Review correctness."})
		}
		p.writeRecord(path, record)
		fill["<path>"] = path
	case "verify":
		path := p.recordPath(brief)
		record := kernel.VerifyRecord{Step: a.Step, Subject: a.Subject.Commit, Returns: []kernel.VerifyReturn{}}
		for _, m := range printedFinding.FindAllStringSubmatch(brief, -1) {
			round, err := strconv.Atoi(m[2])
			p.must(err)
			record.Returns = append(record.Returns, kernel.VerifyReturn{Slot: m[1], Round: round, Finding: m[3], FindingSHA256: m[4], Return: "reproduced"})
		}
		if len(record.Returns) == 0 {
			p.t.Fatalf("verify brief lists no finding: %s", brief)
		}
		p.writeRecord(path, record)
		fill["<path>"] = path
	default:
		p.t.Fatalf("unexpected actor %s: %+v", a.Kind, a)
	}
	p.submit(a, fill)
}

// lensSlots names the non-adversarial slots of the round that the lens step drafts.
// The lens brief does not list the slots, so the actor reads the reviewer roster.
func (p *printedLoop) lensSlots(a kernel.Action) []string {
	p.t.Helper()
	cfg, err := config.Load(p.cwd)
	p.must(err)
	tuple, _, err := p.row(a.Step).Tuple()
	p.must(err)
	kind := "bead"
	if tuple.WorkItem == "" {
		kind = "final_review"
	}
	var round kernel.GateRound
	rows, err := p.f.Store.All()
	p.must(err)
	for _, r := range rows {
		value, has, err := kernel.Decode[kernel.GateRound](r.Metadata, kernel.KeyRound)
		p.must(err)
		if has && value.Lens == a.Step {
			round = value
		}
	}
	if round.Round == 0 {
		p.t.Fatalf("no round names lens %s", a.Step)
	}
	roster, err := kernel.RoundReviewerSlots(cfg, kind, round)
	p.must(err)
	var slots []string
	for _, slot := range roster {
		if slot.Lens != "adversarial" {
			slots = append(slots, slot.Slot)
		}
	}
	return slots
}

// drive applies until the spec is done. Each pass runs every printed action and
// decision; a pass with neither runs its first run entry.
func (p *printedLoop) drive(verdict func(kernel.Action) string) []string {
	p.t.Helper()
	var ran []string
	for pass := 0; pass < 40; pass++ {
		out := p.apply()
		p.allow(out)
		if out.Status == "done" {
			return ran
		}
		for _, a := range out.Actions {
			p.act(a, verdict)
		}
		for _, d := range out.Decisions {
			p.do(d.Submit, map[string]string{"<option>": "fix"})
		}
		if len(out.Actions) == 0 && len(out.Decisions) == 0 {
			if len(out.Run) == 0 {
				p.t.Fatalf("no printed step: %+v", out)
			}
			ran = append(ran, strings.Join(out.Run[0].Argv[1:], " "))
			p.do(out.Run[0].Argv, nil)
		}
	}
	p.t.Fatal("loop did not finish")
	return ran
}

// The spec runs from plan approval to done through printed commands alone, with a
// refused round, its verify and fix, and a passing second round.
func TestS134E2E_PrintedCommandLoop(t *testing.T) {
	t.Parallel()
	p := newPrintedLoop(newKernelE2EWithPlan(t, kernelE2EBinary(t), kerneltest.EmptyRepo(t), ".mindspec", kernelE2EConfig, readyPlan, 1))
	item := p.f.Items[0]
	refused := false
	ran := p.drive(func(a kernel.Action) string {
		if a.Step == item+".review.r1.R1" {
			refused = true
			return "REQUEST_CHANGES"
		}
		return "APPROVE"
	})
	if !refused {
		t.Fatal("the refused slot did not run")
	}
	for kind, want := range map[string]int{"implement": 1, "lens": 3, "review-slot": 9, "verify": 1, "fix": 1} {
		if p.kinds[kind] != want {
			t.Errorf("%s steps %d, want %d: %v", kind, p.kinds[kind], want, p.kinds)
		}
	}
	want := []string{"panel create " + item, "panel create " + item, "complete " + item, "panel create " + p.f.SpecID, "impl approve " + p.f.SpecID}
	if strings.Join(ran, "\n") != strings.Join(want, "\n") {
		t.Fatalf("run entries:\n%s\nwant:\n%s", strings.Join(ran, "\n"), strings.Join(want, "\n"))
	}
	for _, section := range []string{"## Rules (fixer)", "Fix these reproduced findings.", "Finding: R1 round 1"} {
		if !strings.Contains(p.briefs["fix"], section) {
			t.Fatalf("fix brief lacks %q: %s", section, p.briefs["fix"])
		}
	}
	if !strings.Contains(p.briefs["verify"], "Finding: R1 round 1") {
		t.Fatalf("verify brief lacks the refused finding: %s", p.briefs["verify"])
	}
	// The implementer gets the output of the implementation action's brief
	// argv, the work-item brief, which carries the implementer rules alone.
	if argv := p.briefArgv["implement"]; !strings.HasSuffix(argv, " --emit-only") || !strings.Contains(argv, " next ") {
		t.Fatalf("implement brief argv = %q, want the work-item brief", argv)
	}
	implementerRules, err := rules.Section("implementer")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.briefs["implement"], implementerRules) || strings.Contains(p.briefs["implement"], "## Rules (fixer)") {
		t.Fatalf("implement brief lacks the implementer rules or prints the fixer rules: %s", p.briefs["implement"])
	}
}
