package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var panelRefuteCmd = &cobra.Command{
	Use: "refute <slug>", Short: "Record an evidenced refutation of a latest-round finding",
	Args: cobra.ExactArgs(1), SilenceUsage: true,
	RunE: runPanelRefute,
}

func panelVerbFailure(err error) error {
	return guard.NewFailure(termsafe.Escape(err.Error()), "correct the panel arguments or registration, then retry the panel verb")
}

func resolvePanelVerb(cmd *cobra.Command, slug string) (string, panel.Registration, error) {
	if err := validatePanelSlug(slug); err != nil {
		return "", panel.Registration{}, err
	}
	root, err := findRoot()
	if err != nil {
		return "", panel.Registration{}, err
	}
	spec, _ := cmd.Flags().GetString("spec")
	reg, err := findPanelRegistration(root, slug, spec)
	if err != nil {
		return "", reg, err
	}
	if reg.Err != nil {
		return "", reg, reg.Err
	}
	return root, reg, nil
}

func panelVerbSnapshot(dir string) (*panel.Result, error) {
	res, err := panel.Tally(dir)
	if err != nil {
		return nil, err
	}
	if res.PanelErr != nil {
		return nil, res.PanelErr
	}
	if res.Panel == nil {
		return nil, fmt.Errorf("panel registration is missing")
	}
	if res.IntegrityErr != nil {
		return nil, res.IntegrityErr
	}
	if res.RoundMismatch {
		return nil, fmt.Errorf("panel round differs from latest verdict round")
	}
	return res, nil
}

func panelRecordSpec(p panel.Panel) string {
	if p.Spec == "" {
		return "adhoc"
	}
	return p.Spec
}

// panelEvidenceResolves accepts an HTTPS reference, an existing path inside
// the repository, or a commit object. The commit suffix excludes blob objects.
func panelEvidenceResolves(root, evidence string) bool {
	if u, err := url.Parse(evidence); err == nil && u.Scheme == "https" && u.Host != "" {
		return true
	}
	candidate := evidence
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
		if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
			root = resolvedRoot
		}
		rel, err := filepath.Rel(root, resolved)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if _, err := os.Stat(resolved); err == nil {
				return true
			}
		}
	}
	if strings.HasPrefix(evidence, "-") {
		return false
	}
	cmd := exec.Command("git", "-C", root, "cat-file", "-e", evidence+"^{commit}")
	return cmd.Run() == nil
}

func panelAcceptanceExists(root, spec, ac string) bool {
	if !regexp.MustCompile(`^AC-[0-9]+$`).MatchString(ac) || spec == "" {
		return false
	}
	dir, err := workspace.SpecDir(root, spec)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, "spec.md"))
	if err != nil {
		return false
	}
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(ac) + `\b`).Match(data)
}

func refuteFinding(dir, slot string, round int, id string) (panel.Finding, error) {
	v, err := panel.ReadFindingVerdict(dir, slot, round)
	if err != nil {
		return panel.Finding{}, err
	}
	if strings.ToUpper(strings.TrimSpace(v.Verdict)) != panel.VerdictRequestChanges || v.HardBlock {
		return panel.Finding{}, fmt.Errorf("only REQUEST_CHANGES without hard_block can be refuted")
	}
	if id == "" {
		return panel.SlotFinding(v)
	}
	findings, err := panel.Findings(v)
	if err != nil {
		return panel.Finding{}, err
	}
	for _, f := range findings {
		if f.ID == id {
			return f, nil
		}
	}
	return panel.Finding{}, fmt.Errorf("unknown finding id")
}

func runPanelRefute(cmd *cobra.Command, args []string) error {
	root, reg, err := resolvePanelVerb(cmd, args[0])
	if err != nil {
		return panelVerbFailure(err)
	}
	slot, _ := cmd.Flags().GetString("slot")
	round, _ := cmd.Flags().GetInt("round")
	finding, _ := cmd.Flags().GetString("finding")
	ground, _ := cmd.Flags().GetString("ground")
	ac, _ := cmd.Flags().GetString("ac")
	reason, _ := cmd.Flags().GetString("reason")
	evidence, _ := cmd.Flags().GetString("evidence")
	by, _ := cmd.Flags().GetString("by")
	if strings.TrimSpace(by) == "" || strings.TrimSpace(reason) == "" || strings.TrimSpace(evidence) == "" {
		return panelVerbFailure(fmt.Errorf("--by, --reason and --evidence must not be empty"))
	}
	switch ground {
	case "disproven", "not-load-bearing":
	case "out_of_acceptance_scope":
		if !panelAcceptanceExists(root, reg.Panel.Spec, ac) {
			return panelVerbFailure(fmt.Errorf("--ac must name an acceptance criterion in the owning spec"))
		}
	default:
		return panelVerbFailure(fmt.Errorf("--ground must be disproven, not-load-bearing or out_of_acceptance_scope"))
	}
	if !panelEvidenceResolves(root, evidence) {
		return panelVerbFailure(fmt.Errorf("--evidence must resolve to a commit, repo path or https:// URL"))
	}
	// A resolving absolute repo path is stored relative to the repo so the
	// evidence remains portable and passes the ledger's path hygiene check.
	if filepath.IsAbs(evidence) {
		resolved, err := filepath.EvalSymlinks(evidence)
		if err != nil {
			return panelVerbFailure(err)
		}
		if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
			root = resolvedRoot
		}
		relative, err := filepath.Rel(root, resolved)
		if err != nil {
			return panelVerbFailure(err)
		}
		evidence = filepath.ToSlash(relative)
	}
	res, err := panelVerbSnapshot(reg.Dir)
	if err != nil {
		return panelVerbFailure(err)
	}
	if res.Panel.Abandoned {
		return panelVerbFailure(fmt.Errorf("panel is abandoned"))
	}
	f, err := refuteFinding(reg.Dir, slot, round, finding)
	if err != nil {
		return panelVerbFailure(err)
	}
	model := ""
	for _, s := range res.Panel.Roster {
		if s.Slot == slot {
			model = s.Model
		}
	}
	if model == "" {
		return panelVerbFailure(fmt.Errorf("slot must belong to the recorded roster"))
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	ref := panel.Refutation{Slot: slot, Round: round, Finding: f.ID, FindingSHA256: f.SHA256,
		FindingText: f.Text, Ground: ground, AC: ac, Reason: reason, Evidence: evidence, By: by, CreatedAt: now}
	event, err := json.Marshal(panel.RefutationRecord{Record: panel.RecordRefutation, Spec: panelRecordSpec(*res.Panel), Panel: args[0], Refutation: ref})
	if err != nil {
		return panelVerbFailure(err)
	}
	gate := res.Panel.Gate
	if gate == "" {
		gate = "adhoc"
	}
	row, err := json.Marshal(panel.DispositionRow{Record: "disposition", ID: fmt.Sprintf("%s:%s:r%d:%s:%s:refutation", args[0], slot, round, f.ID, f.SHA256[:12]),
		Spec: panelRecordSpec(*res.Panel), Gate: gate, Panel: args[0], Reviewer: slot, Model: model, Severity: "blocking",
		Summary: termsafe.Escape(f.Text), ConvergentWith: []string{}, Disposition: "audited-refuted", EvidenceRef: evidence, CreatedAt: now, Round: &round})
	if err != nil {
		return panelVerbFailure(err)
	}
	err = panel.AppendRecordsWithCheck(reg.Dir, func() error {
		current, err := panelVerbSnapshot(reg.Dir)
		if err != nil {
			return err
		}
		if current.Panel.Abandoned {
			return fmt.Errorf("panel is abandoned")
		}
		latest, err := refuteFinding(reg.Dir, slot, round, finding)
		if err != nil {
			return err
		}
		if latest != f {
			return fmt.Errorf("finding changed; retry against the current verdict")
		}
		res = current
		return nil
	}, event, row)
	if err != nil {
		return panelVerbFailure(err)
	}
	found := false
	for _, old := range res.Panel.Refutations {
		if old.Slot == slot && old.Round == round && old.Finding == f.ID && old.FindingSHA256 == f.SHA256 {
			found = true
		}
	}
	if !found {
		res.Panel.Refutations = append(res.Panel.Refutations, ref)
	}
	if err := panel.WriteSnapshot(reg.Dir, *res.Panel); err != nil {
		return panelVerbFailure(err)
	}
	updated, err := panel.Tally(reg.Dir)
	if err != nil {
		return panelVerbFailure(err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), panel.RuleLine(updated))
	return nil
}

func init() {
	panelRefuteCmd.Flags().String("spec", "", "Select the owning spec when the slug is ambiguous")
	panelRefuteCmd.Flags().String("slot", "", "Roster slot to refute")
	panelRefuteCmd.Flags().Int("round", 0, "Latest verdict round")
	panelRefuteCmd.Flags().String("finding", "", "Finding id ccr[i]; omit to cover the whole slot")
	panelRefuteCmd.Flags().String("ground", "", "disproven, not-load-bearing or out_of_acceptance_scope")
	panelRefuteCmd.Flags().String("ac", "", "Acceptance criterion id required for out_of_acceptance_scope")
	panelRefuteCmd.Flags().String("reason", "", "Reason for the refutation")
	panelRefuteCmd.Flags().String("evidence", "", "Resolving commit, repo path or https:// URL")
	panelRefuteCmd.Flags().String("by", "", "Name of the person recording the refutation")
	panelCmd.AddCommand(panelRefuteCmd)
}
