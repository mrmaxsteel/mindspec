package kernel

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

// State is a fresh read of one epic's recorded work and evidence.
type State struct {
	Store *Store `json:"-"`
	// ExcludedStep preserves holds and gate attempts while a verb omits its own target.
	ExcludedStep  *Record `json:"-"`
	Root          string
	SpecID        string
	SpecDir       string
	Epic          string
	Enrolled      bool
	MarkerMissing bool
	Policy        Policy
	At            time.Time
	Records       map[string]Record
	Items         []string
	Gates         []string
	Steps         []string
	Foreign       []string
	Collisions    []string
	Links         Links
	Graph         map[string]Record
	Merged        map[string]bool
	Refs          map[string]string
	Panels        []panel.Registration
	Ledgers       map[string][][]byte
	Verdicts      map[string]json.RawMessage
}

func ReadState(store *Store, specID, specDir string) (State, error) {
	s := State{Store: store, Root: store.Root, SpecID: specID, SpecDir: specDir, Epic: store.Epic, At: Now().UTC(), Policy: DefaultPolicy, Records: map[string]Record{}, Refs: map[string]string{}, Ledgers: map[string][][]byte{}, Verdicts: map[string]json.RawMessage{}}
	all, err := store.All()
	if err != nil {
		return s, err
	}
	var epic *Record
	for _, r := range all {
		if r.ID == s.Epic {
			copy := r
			epic = &copy
		}
	}
	if epic == nil {
		return s, fmt.Errorf("epic not found")
	}
	links, _, err := Decode[Links](epic.Metadata, KeyLinks)
	if err != nil {
		return s, err
	}
	s.Graph = map[string]Record{}
	s.Merged = map[string]bool{}
	for _, r := range all {
		s.Graph[r.ID] = r
	}
	s.Links = links
	for _, link := range links {
		s.Items = append(s.Items, link.ID)
	}
	policy, present, err := Decode[Policy](epic.Metadata, KeyRun)
	if err != nil {
		return s, err
	}
	if present {
		s.Policy = policy
	}
	subtree := Subtree(all, s.Epic)
	selected := map[string]bool{s.Epic: true}
	for _, id := range s.Items {
		selected[id] = true
	}
	for changed := true; changed; {
		changed = false
		for _, r := range all {
			parent := recordParent(r)
			if !selected[r.ID] && (selected[parent] || kernelid.UnderEpic(r.Key(), s.Epic) || kernelid.UnderEpic(r.ID, s.Epic)) {
				selected[r.ID] = true
				changed = true
			}
		}
	}
	w := kernelid.Witnesses{Epic: s.Epic, RunPresent: present}
	for _, r := range all {
		if selected[r.ID] {
			key := r.Key()
			if _, ok := s.Records[key]; ok {
				return s, collisionError(key)
			}
			s.Records[key] = r
			tuple, parseErr := kernelid.Parse(key)
			have, ok, tupleErr := r.Tuple()
			if subtree[r.ID] && parseErr == nil && tuple.Epic == s.Epic && tupleErr == nil && ok && have == tuple {
				w.Keys = append(w.Keys, key)
			}
		}
	}
	w.Orchestrated, err = kernelid.PanelOrchestrated(specDir)
	if err != nil {
		return s, termsafe.NewWrappedFailure(err, fmt.Sprintf("read panel witnesses in %s: %s", specDir, err), fmt.Sprintf("in %s, repair the named panel.json file, then rerun the command", specDir))
	}
	s.Enrolled, s.MarkerMissing = kernelid.Enrolled(w)
	s.classify()
	if !s.Enrolled {
		return s, nil
	}
	s.Panels, err = panelDirectories(s.Root, specDir)
	if err != nil {
		return s, err
	}
	for _, p := range s.Panels {
		if p.Err != nil {
			return s, termsafe.NewWrappedFailure(p.Err, fmt.Sprintf("read panel in %s: %s", p.Dir, p.Err), fmt.Sprintf("in %s, repair panel.json or move the panel out of the review root, then rerun the command", p.Dir))
		}
		lines, err := panel.ReadLedger(p.Dir)
		if err != nil {
			return s, termsafe.NewWrappedFailure(err, fmt.Sprintf("read panel ledger in %s: %s", p.Dir, err), fmt.Sprintf("in %s, repair the ledger or move it out of the review root, then rerun the command", p.Dir))
		}
		s.Ledgers[p.Dir] = lines
		if p.Panel.Round == 0 {
			continue
		}
		paths, err := filepath.Glob(filepath.Join(p.Dir, "*-round-*.json"))
		if err != nil {
			return s, err
		}
		for _, path := range paths {
			data, err := panel.ReadVerdictFile(path, p.Panel.VerdictSchema)
			if err != nil {
				return s, termsafe.NewWrappedFailure(err, fmt.Sprintf("read verdict in %s: %s", p.Dir, err), fmt.Sprintf("in %s, repair %s, then rerun the command", p.Dir, filepath.Base(path)))
			}
			s.Verdicts[path] = data
		}
	}
	specBranch, err := workspace.SpecBranch(specID)
	if err != nil {
		return s, err
	}
	names := []string{"refs/heads/" + specBranch}
	for _, id := range s.Items {
		branch, err := workspace.BeadBranch(id)
		if err != nil {
			return s, err
		}
		names = append(names, "refs/heads/"+branch)
	}
	cmd := exec.Command("git", append([]string{"for-each-ref", "--format=%(refname) %(objectname)"}, names...)...)
	cmd.Dir = s.Root
	out, err := cmd.Output()
	if err != nil {
		return s, fmt.Errorf("read git refs: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			s.Refs[parts[0]] = parts[1]
		}
	}
	for i, id := range s.Items {
		tip := s.Refs[names[i+1]]
		if tip == "" {
			// Completion retains the landed tip when it removes the branch.
			recorded, _ := s.Records[id].Metadata["mindspec_landed_second_parent"].(string)
			merge, _ := s.Records[id].Metadata["mindspec_landed_merge_sha"].(string)
			if objectID(recorded) && objectID(merge) {
				cmd := exec.Command("git", "rev-list", "--parents", "-n", "1", merge, "--")
				cmd.Dir = s.Root
				out, err := cmd.Output()
				if err != nil {
					return s, fmt.Errorf("read landed merge: %w", err)
				}
				parts := strings.Fields(string(out))
				if len(parts) == 3 && parts[2] == recorded {
					tip = merge
				}
			}
		}
		target := s.Refs["refs/heads/"+specBranch]
		if target == "" {
			// Finalization retains the publication tip after removing the branch.
			publication, _ := epic.Metadata["mindspec_finalize_tip"].(string)
			if objectID(publication) {
				target = publication
			}
		}
		if tip == "" || target == "" {
			continue
		}
		cmd := exec.Command("git", "merge-base", "--is-ancestor", tip, target)
		cmd.Dir = s.Root
		if err := cmd.Run(); err == nil {
			s.Merged[id] = true
		} else if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			return s, fmt.Errorf("read merge state: %w", err)
		}
	}
	return s, nil
}

func (s *State) classify() {
	s.Gates = nil
	s.Steps = nil
	s.Foreign = nil
	s.Collisions = nil
	recorded := map[string]bool{s.Epic: true}
	for _, id := range s.Items {
		recorded[id] = true
	}
	for key, r := range s.Records {
		if recorded[key] {
			continue
		}
		t, err := kernelid.Parse(key)
		have, ok, e := r.Tuple()
		if err != nil || e != nil || !ok || have != t || t.Epic != s.Epic {
			s.Foreign = append(s.Foreign, key)
			if err == nil && t.Epic == s.Epic {
				s.Collisions = append(s.Collisions, key)
			} else if kernelid.UnderEpic(r.ID, s.Epic) {
				s.Collisions = append(s.Collisions, r.ID)
			}
			continue
		}
		if t.Kind == "bead-gate" || t.Kind == "final-gate" {
			if s.ExcludedStep == nil || s.ExcludedStep.Key() != key {
				s.Gates = append(s.Gates, key)
			}
		} else {
			s.Steps = append(s.Steps, key)
		}
	}
	sort.Strings(s.Gates)
	sort.Strings(s.Steps)
	sort.Strings(s.Foreign)
	sort.Strings(s.Collisions)
}

// gateAttempts includes an omitted gate for selection and evidence reads.
// Gates lists only the targets that rules may process.
func (s State) gateAttempts() []string {
	if r := s.ExcludedStep; r != nil {
		t, has, err := r.Tuple()
		if err == nil && has && (t.Kind == "bead-gate" || t.Kind == "final-gate") {
			return append(append([]string{}, s.Gates...), r.Key())
		}
	}
	return s.Gates
}
func (s State) OpenDecisions() []Record {
	var result []Record
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		_, answered := s.Answers()[id]
		if t.Kind == "decision" && r.Status != "closed" && !answered {
			result = append(result, r)
		}
	}
	return result
}
func (s State) Answers() map[string]DecisionAnswer {
	out := map[string]DecisionAnswer{}
	for _, id := range s.Steps {
		r := s.Records[id]
		tuple, ok, e := r.Tuple()
		if e != nil || !ok || tuple.Kind != "decision" {
			continue
		}
		receipt, ok, err := Decode[Receipt](r.Metadata, KeyClose)
		if err == nil && ok && receipt.Answer != nil && ValidReceipt(s.Root, receipt) == nil {
			out[id] = *receipt.Answer
		}
	}
	return out
}
func (s State) Halted() bool {
	for _, answer := range s.Answers() {
		if answer.Option == "halt" {
			return true
		}
	}
	return false
}
func (s State) Clone() State {
	out := s
	data, _ := json.Marshal(s.Records)
	// Unmarshal into a fresh map so overlays cannot mutate the caller.
	out.Records = map[string]Record{}
	_ = json.Unmarshal(data, &out.Records)
	out.Items = append([]string{}, s.Items...)
	out.classify()
	return out
}
