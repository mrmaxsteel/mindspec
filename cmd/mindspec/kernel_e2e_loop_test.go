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
	"github.com/mrmaxsteel/mindspec/internal/panel/paneltest"
	"github.com/mrmaxsteel/mindspec/internal/rules"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
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
	// afterRun runs after each printed run entry, with the entry's words.
	afterRun func(string)
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
		p.slot(a, brief, verdict(a), fill)
		return
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
		p.writeRecord(path, verifyRecordFromBrief(p.t, brief))
		fill["<path>"] = path
	default:
		p.t.Fatalf("unexpected actor %s: %+v", a.Kind, a)
	}
	p.submit(a, fill)
}

// printedFields returns the field names in the "- name: text" lines after the
// label line, and the text of each.
func printedFields(t *testing.T, brief, label string) ([]string, map[string]string) {
	t.Helper()
	lines := strings.Split(brief, "\n")
	start := -1
	for i, line := range lines {
		if line == label {
			start = i + 1
		}
	}
	if start < 0 {
		t.Fatalf("verify brief lacks %q: %s", label, brief)
	}
	var names []string
	text := map[string]string{}
	for _, line := range lines[start:] {
		name, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), ": ")
		if !strings.HasPrefix(line, "- ") || !ok {
			break
		}
		names = append(names, name)
		text[name] = value
	}
	return names, text
}

// verifyRecordFromBrief builds the verify record from the field list and the
// values that the verify brief prints. Each finding is returned as reproduced.
func verifyRecordFromBrief(t *testing.T, brief string) map[string]any {
	t.Helper()
	top, _ := printedFields(t, brief, "Return a record with these fields:")
	fields, text := printedFields(t, brief, "Each entry in returns has these fields:")
	if !strings.Contains(text["finding"], "a string, ccr[<i>] or veto") {
		t.Fatalf("verify brief does not give the finding value form: %s", brief)
	}
	var returns []map[string]any
	for _, m := range printedFinding.FindAllStringSubmatch(brief, -1) {
		round, err := strconv.Atoi(m[2])
		if err != nil {
			t.Fatal(err)
		}
		values := map[string]any{"slot": m[1], "round": round, "finding": m[3], "finding_sha256": m[4], "return": "reproduced"}
		entry := map[string]any{}
		for _, name := range fields {
			v, ok := values[name]
			if !ok {
				t.Fatalf("verify brief names an unknown entry field %q", name)
			}
			entry[name] = v
		}
		returns = append(returns, entry)
	}
	if len(returns) == 0 {
		t.Fatalf("verify brief lists no finding: %s", brief)
	}
	values := map[string]any{"step": printedValue(t, brief, "Step: "), "subject": printedValue(t, brief, "Subject: "), "returns": returns}
	record := map[string]any{}
	for _, name := range top {
		v, ok := values[name]
		if !ok {
			t.Fatalf("verify brief names an unknown record field %q", name)
		}
		record[name] = v
	}
	return record
}

// printedValue returns the rest of the first line that starts with prefix.
func printedValue(t *testing.T, text, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	t.Fatalf("printed text lacks %q: %s", prefix, text)
	return ""
}

// printedLine returns the line after the label line in a printed text.
func printedLine(text, label string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == label && i+1 < len(lines) {
			return lines[i+1]
		}
	}
	return ""
}

// shellWords splits a printed command line with single-quoted words.
func shellWords(t *testing.T, line string) []string {
	t.Helper()
	var words []string
	var word strings.Builder
	in, quoted := false, false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\'':
			in, quoted = !in, true
		case c == ' ' && !in:
			if word.Len() > 0 || quoted {
				words = append(words, word.String())
			}
			word.Reset()
			quoted = false
		case c == '\\' && !in && i+1 < len(line):
			i++
			word.WriteByte(line[i])
		default:
			word.WriteByte(c)
		}
	}
	if in {
		t.Fatalf("unclosed quote in printed command: %q", line)
	}
	if word.Len() > 0 || quoted {
		words = append(words, word.String())
	}
	return words
}

// slot runs only the commands that the slot prompt prints: the attempt
// command when the slot has no attempt, then the submit command. The verdict
// goes to the path that the submit command names. No commit is made by hand.
func (p *printedLoop) slot(a kernel.Action, prompt, word string, fill map[string]string) {
	p.t.Helper()
	run := func(line string) string {
		argv := shellWords(p.t, line)
		p.printed[strings.Join(argv, "\x00")] = true
		return p.do(argv, fill)
	}
	if checkout := printedLine(prompt, "Make the slot attempt through this command:"); checkout != "" {
		run(checkout)
		prompt = run(printedLine(prompt, "Then run this command for the submission command:"))
	}
	line := printedLine(prompt, "Submit the verdict JSON through this command:")
	argv := shellWords(p.t, line)
	if len(argv) != 7 || strings.Join(argv[:4], " ") != "mindspec panel submit --file" || argv[5] != "--lease" || argv[6] != "<token>" {
		p.t.Fatalf("slot prompt prints no leased submit command: %q\n%s", line, prompt)
	}
	tuple, _, err := p.row(a.Step).Tuple()
	p.must(err)
	v := paneltest.Verdict(word)
	v.Slot = fmt.Sprintf("R%d", tuple.Slot)
	v.Round = a.Round
	v.ReviewedSHA = a.Subject.Commit
	v.Model = "test-model"
	p.must(os.WriteFile(argv[4], paneltest.Bytes(p.t, v), 0600))
	if !strings.Contains(prompt, "Replace <token> with the token that mindspec lease '"+a.Step+"' printed.") {
		p.t.Fatalf("slot prompt does not name the lease step %s:\n%s", a.Step, prompt)
	}
	if p.kinds["review-slot"] == 1 {
		// Without the token the refusal prints the submit command to run.
		out, code := p.run(argv[1:5]...)
		want := "\nrecovery: mindspec panel submit --file '" + argv[4] + "' --lease <token> (the token that mindspec lease '" + a.Step + "' printed)"
		if code == 0 || !strings.HasSuffix(strings.TrimRight(out, "\n"), want) || strings.Count(out, "recovery:") != 1 {
			p.t.Fatalf("submit without a token: exit %d\n%s\nwant suffix %q", code, out, want)
		}
	}
	run(line)
	if status := p.row(a.Step).Status; status != "closed" {
		p.t.Fatalf("printed slot submit left %s %s", a.Step, status)
	}
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
			entry := strings.Join(out.Run[0].Argv[1:], " ")
			ran = append(ran, entry)
			p.do(out.Run[0].Argv, nil)
			if p.afterRun != nil {
				p.afterRun(entry)
			}
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
	// A set-up repository ignores the panel isolation root, where slot attempts live.
	exclude, err := os.ReadFile(filepath.Join(p.f.Root, ".git", "info", "exclude"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	p.write(p.f.Root, ".git/info/exclude", string(exclude)+"\n.mindspec-scratch\n")
	item := p.f.Items[0]
	clones := 0
	p.afterRun = func(entry string) {
		if strings.HasPrefix(entry, "complete ") || strings.HasPrefix(entry, "impl approve ") {
			clones++
			p.cloneReading(entry)
		}
	}
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
	if clones != 2 {
		t.Fatalf("clone readings %d, want one after complete and one after impl approve", clones)
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

// reading returns the next-action reading of the spec against the
// repository at root and the shared tracker store, and checks each receipt.
func (p *printedLoop) reading(root string) (kernel.Output, map[string]error) {
	p.t.Helper()
	dir, err := workspace.SpecDir(root, p.f.SpecID)
	p.must(err)
	s, err := kernel.ReadState(kernel.NewStore(root, p.f.Epic, kerneltest.Runner(p.t, p.f.Root)), p.f.SpecID, dir)
	p.must(err)
	result, err := kernel.Pass(kernel.Rules(), s, false)
	p.must(err)
	receipts := map[string]error{}
	for key, r := range s.Records {
		receipt, has, err := kernel.Decode[kernel.Receipt](r.Metadata, kernel.KeyClose)
		p.must(err)
		if has && receipt.Commit != "" {
			receipts[key] = kernel.ValidReceipt(root, receipt, r.Metadata)
		}
	}
	return kernel.OutputForPass(result, s, false), receipts
}

// cloneReading clones the repository with no local shortcut, as another
// machine gets it by the normal push and fetch, and checks that the clone
// reads the spec as the publishing repository does.
func (p *printedLoop) cloneReading(entry string) {
	p.t.Helper()
	want, wantReceipts := p.reading(p.f.Root)
	clone := filepath.Join(p.t.TempDir(), "clone")
	args := []string{"clone", "--quiet", "--no-local"}
	branch, err := workspace.SpecBranch(p.f.SpecID)
	p.must(err)
	if _, err := p.f.Command("git", "-C", p.f.Root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		args = append(args, "--branch", branch)
	}
	p.git(p.f.Root, append(args, p.f.Root, clone)...)
	got, gotReceipts := p.reading(clone)
	slots := 0
	for key, err := range wantReceipts {
		if err != nil {
			p.t.Fatalf("after %s: publishing repository receipt of %s is not valid: %v", entry, key, err)
		}
		if gotReceipts[key] != nil {
			p.t.Fatalf("after %s: clone receipt of %s is not valid: %v", entry, key, gotReceipts[key])
		}
		if strings.Contains(key, ".r") && strings.Contains(key, ".R") {
			slots++
		}
	}
	if slots == 0 || len(gotReceipts) != len(wantReceipts) {
		p.t.Fatalf("after %s: slot receipts %d, clone receipts %d of %d", entry, slots, len(gotReceipts), len(wantReceipts))
	}
	for _, flag := range got.Flags {
		if flag.Code == "unreceipted-close" || flag.Code == "forced-close" {
			p.t.Fatalf("after %s: clone flags %s %s", entry, flag.Code, flag.Step)
		}
	}
	wantFlags, err := json.Marshal(want.Flags)
	p.must(err)
	gotFlags, err := json.Marshal(got.Flags)
	p.must(err)
	// Before finalization the panel ledgers are files of the spec worktree that
	// no commit holds yet, and the revision hashes them. The merge to main
	// carries them, so the revision must match from then on.
	sameRevision := got.Revision == want.Revision || !strings.HasPrefix(entry, "impl approve ")
	if got.Status != want.Status || !sameRevision || string(gotFlags) != string(wantFlags) {
		p.t.Fatalf("after %s: clone reads %s %s %s, publishing repository reads %s %s %s", entry, got.Status, got.Revision, gotFlags, want.Status, want.Revision, wantFlags)
	}
}
