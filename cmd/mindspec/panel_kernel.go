package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/frontmatter"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/validate"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

func panelStoreFailure(err error) error {
	return guard.NewFailure("panel tracker store is unavailable: "+termsafe.Escape(err.Error()), "restore the tracker store and bd on PATH, then run bd list --all --json")
}

func panelHasEnrollment(root, spec string) bool {
	roots := []string{root, filepath.Join(root, ".mindspec")}
	entries, _ := os.ReadDir(workspace.SpecsDir(root))
	for _, entry := range entries {
		if entry.IsDir() && idvalidate.SpecID(entry.Name()) == nil {
			roots = append(roots, filepath.Join(workspace.SpecsDir(root), entry.Name()))
		}
	}
	for _, reg := range panel.Scan(roots...) {
		if reg.Err == nil && reg.Panel.Orchestrated && (spec == "" || reg.Panel.Spec == spec) {
			return true
		}
	}
	return false
}

// panelCountsDocumentRounds keeps document review available without the tracker.
func panelCountsDocumentRounds(root, specDir, spec string) bool {
	if !strings.EqualFold(frontmatter.StatusFromPath(filepath.Join(specDir, "plan.md")), "Approved") {
		return true
	}
	if panelHasEnrollment(root, spec) {
		return true
	}
	// An approved plan without a local witness may still be enrolled in the store.
	// A failed lookup does not prevent document review.
	s, err := panelKernelStateFn(root, spec)
	return err == nil && s != nil && s.Enrolled
}

// panelKernelStateFn resolves enrollment without changing the store.
var panelKernelStateFn = func(root, spec string) (*kernel.State, error) {
	available := os.Getenv("BEADS_DIR") != ""
	for _, name := range []string{"config.yaml", "metadata.json", "dolt", "beads.db", "redirect"} {
		if _, err := os.Stat(filepath.Join(root, ".beads", name)); err == nil {
			available = true
		} else if !os.IsNotExist(err) {
			return nil, panelStoreFailure(err)
		}
	}
	if !available {
		if panelHasEnrollment(root, spec) {
			return nil, panelStoreFailure(fmt.Errorf("the enrolled panel requires its tracker store"))
		}
		return nil, nil
	}
	store := kernel.NewStore(root, "", nil)
	rows, err := store.All()
	if err != nil {
		return nil, panelStoreFailure(err)
	}
	var states []kernel.State
	for _, r := range rows {
		if r.Type != "epic" {
			continue
		}
		id, e := phase.ResolveSpec(phase.EpicInfo{ID: r.ID, Title: r.Title, Metadata: r.Metadata})
		if e != nil {
			continue
		}
		if spec != "" && id != spec && !strings.HasPrefix(id, spec+"-") {
			continue
		}
		if spec == "" && r.Status == "closed" {
			continue
		}
		dir, e := workspace.SpecDir(root, id)
		if e != nil {
			return nil, e
		}
		st := kernel.NewStore(root, r.ID, nil)
		s, e := kernel.ReadState(st, id, dir)
		if e != nil {
			return nil, panelVerbFailure(e)
		}
		if s.Enrolled {
			states = append(states, s)
		}
	}
	if len(states) == 0 {
		return nil, nil
	}
	if len(states) != 1 {
		return nil, panelVerbFailure(fmt.Errorf("select one enrolled spec with --spec"))
	}
	return &states[0], nil
}

func gateBase(s kernel.State, gate string) string {
	t, _, _ := s.Records[gate].Tuple()
	return t.Gate
}
func kernelPanelGate(s kernel.State, p panel.Panel) string {
	base := s.Epic + ".final"
	if p.BeadID != nil {
		base = *p.BeadID + ".review"
	}
	best, attempt := base, 0
	for _, id := range s.Gates {
		t, _, _ := s.Records[id].Tuple()
		if t.Gate == base && t.Attempt > attempt {
			best, attempt = id, t.Attempt
		}
	}
	return best
}
func panelKernelGuard(s kernel.State, gate string, limit bool) error {
	if s.Halted() {
		for id, a := range s.Answers() {
			if a.Option == "halt" {
				return guard.NewFailure(fmt.Sprintf("spec is halted by %s", termsafe.Escape(id)), "mindspec next-action")
			}
		}
	}
	if len(s.Collisions) > 0 {
		return panelVerbFailure(fmt.Errorf("id-collision: %s; remove that bead with bd", s.Collisions[0]))
	}
	for _, r := range s.OpenDecisions() {
		t, _, _ := r.Tuple()
		if t.Gate == gateBase(s, gate) && (t.Cause == "reject" || limit) {
			answer := "fix"
			if t.Cause == "limit" {
				answer = "continue:1"
			}
			return guard.NewFailure(fmt.Sprintf("answer decision %s first", r.Key()), fmt.Sprintf("mindspec submit %s --answer %s", r.Key(), answer))
		}
	}
	return nil
}
func refreshPanelState(s kernel.State) (kernel.State, error) {
	return kernel.ReadState(s.Store, s.SpecID, s.SpecDir)
}

// panelCreateKernelGate selects lifecycle shorthand from the approved spec tree.
// It reads tracker state to check enrollment for an inferred final review.
func panelCreateKernelGate(root, slug, spec, target, item string) string {
	if item != "" || strings.HasPrefix(target, "bead/") {
		return "bead"
	}
	if n, err := strconv.Atoi(slug); err == nil && n > 0 {
		return "bead"
	}
	if spec != "" && slug == spec {
		return "final_review"
	}
	specIDs := []string{}
	if spec != "" {
		if idvalidate.SpecID(spec) != nil {
			return ""
		}
		specIDs = append(specIDs, spec)
	} else {
		roots := []string{root}
		entries, _ := os.ReadDir(workspace.DefaultWorktreesDir(root))
		for _, entry := range entries {
			if entry.IsDir() {
				roots = append(roots, filepath.Join(workspace.DefaultWorktreesDir(root), entry.Name()))
			}
		}
		seen := map[string]bool{}
		for _, tree := range roots {
			entries, _ := os.ReadDir(workspace.SpecsDir(tree))
			for _, entry := range entries {
				if entry.IsDir() && idvalidate.SpecID(entry.Name()) == nil && !seen[entry.Name()] {
					specIDs = append(specIDs, entry.Name())
					seen[entry.Name()] = true
				}
			}
		}
	}
	dirs := []string{}
	for _, id := range specIDs {
		if dir, err := workspace.SpecDir(root, id); err == nil {
			dirs = append(dirs, dir)
		}
	}
	for _, dir := range dirs {
		if slug == filepath.Base(dir) {
			s, err := panelKernelStateFn(root, slug)
			if err == nil && s != nil && s.Enrolled && s.SpecID == slug {
				return "final_review"
			}
		}
		data, err := os.ReadFile(filepath.Join(dir, "plan.md"))
		if err != nil {
			continue
		}
		fm, err := validate.ParsePlanFrontmatter(string(data))
		if err != nil {
			continue
		}
		for _, id := range fm.BeadIDs {
			if slug == id {
				return "bead"
			}
		}
	}
	return ""
}

func tryKernelPanelCreate(cmd *cobra.Command, slug, spec, kind string) (bool, error) {
	if kind != "" && kind != "bead" && kind != "final_review" {
		return false, nil
	}
	root, err := findPanelRoot(config.IsolationRootFor)
	if err != nil {
		return false, nil
	}
	if kind == "" {
		target, _ := cmd.Flags().GetString("target")
		item, _ := cmd.Flags().GetString("bead")
		kind = panelCreateKernelGate(root, slug, spec, target, item)
		if kind == "" {
			return false, nil
		}
	}
	s, err := panelKernelStateFn(root, spec)
	if err != nil {
		return true, err
	}
	if s == nil {
		return false, nil
	}
	return true, phase.WithPhaseLock(s.Root, func() error {
		fresh, e := refreshPanelState(*s)
		if e != nil {
			return e
		}
		s = &fresh
		target, _ := cmd.Flags().GetString("target")
		item, _ := cmd.Flags().GetString("bead")
		if item == "" && strings.HasPrefix(target, "bead/") {
			derived, err := panel.CreateBeadID("bead", target, "")
			if err != nil {
				return panelVerbFailure(err)
			}
			item = *derived
		}
		if item == "" && kind != "final_review" {
			for i, id := range s.Items {
				if slug == id || slug == strconv.Itoa(i+1) {
					item = id
					break
				}
			}
		}
		if item == "" && kind != "final_review" && slug != s.SpecID {
			return panelVerbFailure(fmt.Errorf("select a recorded work item or the final review gate"))
		}
		if kind == "final_review" && item != "" {
			return panelVerbFailure(fmt.Errorf("--bead is only valid for a bead gate"))
		}
		if kind == "bead" && item == "" {
			return panelVerbFailure(fmt.Errorf("a bead gate requires a recorded work item"))
		}
		p := panel.Panel{Gate: "final_review"}
		expected, e := workspace.SpecBranch(s.SpecID)
		if e != nil {
			return panelVerbFailure(e)
		}
		if item != "" {
			found := false
			for _, id := range s.Items {
				found = found || id == item
			}
			if !found {
				return panelVerbFailure(fmt.Errorf("work item is not in the approved plan"))
			}
			p.BeadID = &item
			p.Gate = "bead"
			expected, e = workspace.BeadBranch(item)
			if e != nil {
				return panelVerbFailure(e)
			}
		}
		gate := kernelPanelGate(*s, p)
		if e = panelKernelGuard(*s, gate, true); e != nil {
			return e
		}
		if target != "" && target != expected {
			return panelVerbFailure(fmt.Errorf("target must be %s", expected))
		}
		// Other steps reduce first. The named gate remains the verb's responsibility.
		if _, e = kernel.PassExcept(kernel.Rules(), *s, true, gate); e != nil {
			return e
		}
		fresh, e = refreshPanelState(*s)
		if e != nil {
			return e
		}
		s = &fresh
		if e = panelKernelGuard(*s, gate, true); e != nil {
			return e
		}
		g := s.Records[gate]
		if g.Status == "closed" {
			return panelVerbFailure(fmt.Errorf("gate %s is closed", gate))
		}
		count, e := kernel.RoundsForGate(*s, gate)
		if e != nil {
			return e
		}
		n := count + 1
		if n > kernel.Allowance(*s, gate) {
			return panelVerbFailure(fmt.Errorf("round %d exceeds the allowance for %s", n, gate))
		}
		round, ok, e := kernel.Decode[kernel.GateRound](g.Metadata, kernel.KeyRound)
		if e != nil {
			return e
		}
		registered, has, e := kernel.Decode[kernel.RegistrationRecord](g.Metadata, kernel.KeyRegistration)
		if e != nil {
			return e
		}
		if !ok || round.Round != n || (has && registered.Round >= round.Round) {
			return guard.NewFailure(fmt.Sprintf("round %d is not authorized for %s", n, gate), fmt.Sprintf("mindspec next-action %s --apply", shellQuoteTarget(idrender.Spec(s.SpecID))))
		}
		if requested, _ := cmd.Flags().GetInt("round"); cmd.Flags().Changed("round") && requested != round.Round {
			return panelVerbFailure(fmt.Errorf("the authorized round is %d", round.Round))
		}
		for _, id := range s.Steps {
			r := s.Records[id]
			t, _, _ := r.Tuple()
			if t.Gate == gateBase(*s, gate) && t.Kind == "review-slot" && t.Round == round.Round && r.Status != "closed" {
				return guard.NewFailure(fmt.Sprintf("slot %s is open", id), fmt.Sprintf("mindspec panel launch %s R%d --spec %s", shellQuoteTarget(slug), t.Slot, shellQuoteTarget(idrender.Spec(s.SpecID))))
			}
		}
		repanel, rep, e := kernel.Decode[kernel.RePanel](g.Metadata, kernel.KeyRePanel)
		if e != nil {
			return e
		}
		size := round.SlotCount
		if rep {
			size = repanel.SlotCount
			round.Paths = repanel.Paths
		}
		if requested, _ := cmd.Flags().GetInt("reviewers"); cmd.Flags().Changed("reviewers") && requested != size {
			return panelVerbFailure(fmt.Errorf("the authorized round requires %d reviewers", size))
		}
		cfg, e := config.Load(s.Root)
		if e != nil {
			return e
		}
		selected, e := cfg.PanelGateReviewerSlots(p.Gate, size)
		if e != nil {
			return e
		}
		roster := make([]panel.RosterSlot, len(selected))
		slots := make([]string, len(selected))
		for i, r := range selected {
			roster[i] = panel.RosterSlot{Slot: r.Slot, Family: r.Family, Model: r.Model, Lens: r.Lens}
			slots[i] = r.Slot
		}
		panelRoot := workspace.TreeRootForSpecDir(s.SpecDir)
		if panelRoot == "" {
			panelRoot = s.Root
		}
		dir, specDir, e := panelDirFor(panelRoot, s.SpecID, slug, p.Gate)
		if e != nil {
			return e
		}
		if e = checkPanelSlugOwner(s.Root, slug, s.SpecID, p.Gate, dir); e != nil {
			return e
		}
		digest, e := panelSubjectReader(p.Gate, specDir, lsTreeForPanelFn)(panel.PanelDirScanRoot(dir), round.Subject, panel.Subject(p.Gate, specDir))
		if e != nil {
			return e
		}
		regions, e := resolvePanelBriefRegions(s.Root, specDir, s.SpecID, round.Subject, p.Gate, cfg)
		if e != nil {
			return e
		}
		brief, _ := cmd.Flags().GetString("brief-file")
		if cmd.Flags().Changed("brief-file") && brief == "" {
			return panelVerbFailure(fmt.Errorf("brief file must be an absolute path"))
		}
		overrides, _ := cmd.Flags().GetStringArray("reviewer")
		in := panel.CreateInput{Orchestrated: true, KernelGate: gate, Scope: round.Paths, Spec: s.SpecID, BeadID: p.BeadID, Target: expected, Gate: p.Gate, Round: round.Round, ReviewedHeadSHA: round.Subject, SubjectDigest: digest, Roster: roster, ExpectedReviewers: size, ReviewersOverride: true, ReviewerOverrides: overrides, BriefFile: brief, BriefRegions: regions, HasRunner: func(f string) bool { _, ok := cfg.RunnerFor(f); return ok }}
		if e = panel.CheckCreate(dir, in); e != nil {
			return panelCreateFailure(e, slug, in)
		}
		release, e := cleanupPriorRound(s.Root, dir, round.Round)
		if e != nil {
			return e
		}
		defer release()
		if e = panel.Create(dir, in); e != nil {
			return panelCreateFailure(e, slug, in)
		}
		directories := []string{}
		for _, path := range kernel.RegisteredPanelPaths(*s, gate) {
			directories = append(directories, strings.TrimSuffix(path, "/"))
		}
		recordRoot := workspace.TreeRootForSpecDir(specDir)
		if recordRoot == "" {
			recordRoot = s.Root
		}
		rel, e := filepath.Rel(recordRoot, dir)
		if e != nil {
			return e
		}
		directories = append(directories, filepath.ToSlash(rel))
		sort.Strings(directories)
		directories = slices.Compact(directories)
		registration := kernel.RegistrationRecord{Slug: slug, Round: round.Round, Subject: round.Subject, Slots: slots, Directories: directories}
		if e = s.Store.WriteMetadata(gate, kernel.Metadata(kernel.KeyRegistration, registration)); e != nil {
			return e
		}
		fresh, e = refreshPanelState(*s)
		if e != nil {
			return e
		}
		if _, e = kernel.Pass(kernel.Rules(), fresh, true); e != nil {
			return e
		}
		fmt.Fprintf(cmd.OutOrStdout(), "panel %s registered: round %d, %d expected reviewer(s), reviewed_head_sha %s\n", slug, round.Round, size, round.Subject)
		return nil
	})
}

func withKernelPanelVerb(cmd *cobra.Command, slug string, rebind bool, mutate func() error) error {
	root, reg, err := resolvePanelVerb(cmd, slug)
	if err != nil {
		return mutate()
	}
	if reg.Panel.Gate != "bead" && reg.Panel.Gate != "final_review" {
		return mutate()
	}
	s, err := panelKernelStateFn(root, reg.Panel.Spec)
	if err != nil {
		return err
	}
	if s == nil {
		return mutate()
	}
	return phase.WithPhaseLock(s.Root, func() error {
		fresh, e := refreshPanelState(*s)
		if e != nil {
			return e
		}
		s = &fresh
		gate := kernelPanelGate(*s, reg.Panel)
		if e = panelKernelGuard(*s, gate, false); e != nil {
			return e
		}
		if !rebind {
			if _, e = kernel.PassExcept(kernel.Rules(), *s, true, gate); e != nil {
				return e
			}
		}
		if e = mutate(); e != nil {
			return e
		}
		fresh, e = refreshPanelState(*s)
		if e != nil {
			return e
		}
		if !rebind {
			if cmd.Name() == "defer" {
				finding, _ := cmd.Flags().GetString("finding")
				if finding == "" {
					finding = "slot"
				}
				to, _ := cmd.Flags().GetString("bead")
				slot, _ := cmd.Flags().GetString("slot")
				n, _ := cmd.Flags().GetInt("round")
				record := kernel.DeferRecord{Finding: finding, To: to, Reason: "panel defer", At: kernel.Now().UTC(), Panel: slug, Round: n, Slot: slot}
				history, has, err := kernel.Decode[[]kernel.DeferRecord](fresh.Records[gate].Metadata, kernel.KeyDeferrals)
				if err != nil {
					return err
				}
				if !has {
					old, ok, err := kernel.Decode[kernel.DeferRecord](fresh.Records[gate].Metadata, kernel.KeyDefer)
					if err != nil {
						return err
					}
					if ok {
						history = append(history, old)
					}
				}
				duplicate := false
				for _, old := range history {
					duplicate = duplicate || (old.Panel == record.Panel && old.Round == record.Round && old.Slot == record.Slot && old.Finding == record.Finding && old.To == record.To)
				}
				if !duplicate {
					history = append(history, record)
					if e = fresh.Store.WriteMetadata(gate, map[string]any{kernel.KeyDefer: record, kernel.KeyDeferrals: history}); e != nil {
						return e
					}
				}
			}
			if e = closeRetalliedChildren(fresh, gate, reg.Dir); e != nil {
				return e
			}
			fresh, e = refreshPanelState(fresh)
			if e != nil {
				return e
			}
		}
		_, e = kernel.Pass(kernel.Rules(), fresh, true)
		return e
	})
}

func closeRetalliedChildren(s kernel.State, gate, dir string) error {
	res, err := panel.Tally(dir)
	if err != nil {
		return err
	}
	round, ok, err := kernel.Decode[kernel.GateRound](s.Records[gate].Metadata, kernel.KeyRound)
	if err != nil {
		return err
	}
	if res.Panel == nil {
		return panelVerbFailure(fmt.Errorf("panel registration is missing"))
	}
	if !ok || !res.PassRule(res.Panel.ReviewedHeadSHA == round.Subject) {
		return nil
	}
	// Refuse a foreign child before any close writes.
	var children []string
	for _, r := range s.Records {
		if r.Status == "closed" {
			continue
		}
		t, owned, e := r.Tuple()
		if r.Parent == s.Records[gate].ID && (!owned || e != nil || t.Epic != s.Epic) {
			children = append(children, r.ID)
		}
	}
	if len(children) > 0 {
		sort.Strings(children)
		return panelVerbFailure(fmt.Errorf("foreign-child-open: %s; close or re-parent those children with bd", strings.Join(children, ", ")))
	}
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if r.Status == "closed" || t.Gate != gateBase(s, gate) || (t.Kind != "fix" && !(t.Kind == "decision" && t.Cause == "limit")) {
			continue
		}
		for _, child := range s.Records {
			if child.Parent == r.ID && child.Status != "closed" {
				children = append(children, child.ID)
			}
		}
	}
	if len(children) > 0 {
		sort.Strings(children)
		return panelVerbFailure(fmt.Errorf("open children %s prevent close", strings.Join(children, ", ")))
	}
	for _, id := range s.Steps {
		r := s.Records[id]
		t, _, _ := r.Tuple()
		if r.Status == "closed" || t.Gate != gateBase(s, gate) || (t.Kind != "fix" && !(t.Kind == "decision" && t.Cause == "limit")) {
			continue
		}

		if err = kernel.WithCASRetry(func() error {
			current, e := s.Store.Lookup(id)
			if e != nil {
				return e
			}
			if e = s.Store.WriteReceipt(id, kernel.Receipt{Kind: "superseded", Retally: "passing-tally"}); e != nil {
				return e
			}
			if current.Status == "closed" {
				return nil
			}
			return s.Store.SetStatusIf(id, current.Status, "closed")
		}); err != nil {
			return err
		}
	}
	return nil
}
