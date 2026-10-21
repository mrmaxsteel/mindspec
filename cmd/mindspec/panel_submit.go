package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var panelSubmitMetadataFn = bead.GetMetadata

var panelSubmitCmd = &cobra.Command{
	Use:          "submit [slug] --file <path|-> [--slot S --round N --spec ID]",
	Short:        "Validate and submit a slot verdict",
	SilenceUsage: true,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runPanelSubmit,
}

// resolveEffectivePanelSlot is called under Submit's ledger lock. It uses the
// same config load for runner kind, source and effective model.
func resolveEffectivePanelSlot(cfg *config.Config, dir string, p panel.Panel, slot panel.RosterSlot) (panel.SubmitResolution, error) {
	family, model := slot.Family, slot.Model
	if family == "" {
		family = "harness"
	}
	lines, err := panel.ReadLedger(dir)
	if err != nil {
		return panel.SubmitResolution{}, err
	}
	for _, line := range lines {
		var sub panel.SubstitutionRecord
		if err := json.Unmarshal(line, &sub); err != nil {
			return panel.SubmitResolution{}, err
		}
		if sub.Record == panel.RecordSubstitution && sub.Round == p.Round && sub.Slot == slot.Slot {
			family, model = sub.ToFamily, sub.Model
		}
	}
	runner, ok := cfg.RunnerFor(family)
	if !ok {
		return panel.SubmitResolution{}, guard.NewFailure("no runner for family "+termsafe.Escape(family), "add a runner for the family to panel.runners")
	}
	return panel.SubmitResolution{Family: family, Model: model, RunnerKind: runner.Kind, RunnerSource: cfg.RunnerSource(family)}, nil
}

func runPanelSubmit(cmd *cobra.Command, args []string) error {
	file, _ := cmd.Flags().GetString("file")
	if file == "" {
		return guard.NewFailure("--file is required", "select a verdict file or use --file -")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	fromCWD, hasCWD, err := workspace.FindSlotRoot(cwd, config.IsolationRootFor)
	if err != nil {
		return err
	}
	var fromFile workspace.SlotRoot
	hasFile := false
	if filepath.IsAbs(file) {
		fromFile, hasFile, err = workspace.FindSlotRoot(filepath.Dir(file), config.IsolationRootFor)
		if err != nil {
			return err
		}
	}
	if hasCWD && hasFile && fromCWD.Dir != fromFile.Dir {
		return guard.NewFailure(fmt.Sprintf("cwd slot %s differs from file slot %s", termsafe.Escape(fromCWD.Dir), termsafe.Escape(fromFile.Dir)), "submit a file from the cwd slot")
	}
	identity, inferred := fromCWD, hasCWD
	if !inferred {
		identity, inferred = fromFile, hasFile
	}
	slug := ""
	if len(args) > 0 {
		slug = args[0]
	}
	slot, _ := cmd.Flags().GetString("slot")
	spec, _ := cmd.Flags().GetString("spec")
	round, _ := cmd.Flags().GetInt("round")
	root := ""
	if inferred {
		pathSpec := identity.Spec
		if pathSpec == "adhoc" {
			pathSpec = ""
		}
		if slug != "" && slug != identity.Slug || cmd.Flags().Changed("slot") && slot != identity.Slot || cmd.Flags().Changed("spec") && spec != pathSpec || cmd.Flags().Changed("round") && round != identity.Round {
			return guard.NewFailure("explicit target differs from the slot directory", "use the target recorded in slot.json")
		}
		root, slug, slot, spec, round = identity.MainRoot, identity.Slug, identity.Slot, pathSpec, identity.Round
	} else {
		if slug == "" || slot == "" || round < 1 {
			return guard.NewFailure("submit needs a slug, --slot and --round outside a slot directory", "use an absolute --file path in a slot directory, or give the full target")
		}
		root, err = findPanelRoot(config.IsolationRootFor)
		if err != nil {
			return err
		}
	}
	if err := validatePanelSlug(slug); err != nil {
		return err
	}
	reg, err := findPanelRegistration(root, slug, spec)
	if err != nil {
		return err
	}
	if reg.Err != nil {
		return reg.Err
	}
	if inferred && (reg.Panel.Round != round || reg.Panel.Spec != spec) {
		return guard.NewFailure("slot directory differs from the current panel", "use mindspec panel checkout for the current round")
	}
	iso, err := config.IsolationRootFor(root)
	if err != nil {
		return err
	}
	layout, err := panel.NewSlotLayout(root, iso, reg.Panel.Spec, slug, round, slot)
	if err != nil {
		return err
	}
	var reader io.Reader = cmd.InOrStdin()
	if file != "-" {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
	}
	data, err := io.ReadAll(io.LimitReader(reader, panel.MaxVerdictBytes+1))
	if err != nil {
		return err
	}
	target := panel.SubmitTarget{Dir: reg.Dir, Spec: spec, Panel: slug, Slot: slot, Round: round, File: file, SlotDir: layout.Dir}
	_, err = panel.Submit(target, data, func(p panel.Panel, roster panel.RosterSlot) (panel.SubmitResolution, error) {
		cfg, err := config.Load(root)
		if err != nil {
			return panel.SubmitResolution{}, err
		}
		resolved, err := resolveEffectivePanelSlot(cfg, reg.Dir, p, roster)
		if err != nil {
			return resolved, err
		}
		beadID := ""
		if p.BeadID != nil {
			beadID = *p.BeadID
		}
		resolved.Landed, err = landedBindingFor(panelSubmitMetadataFn, beadID, slug, round)
		return resolved, err
	})
	if err != nil {
		return err
	}
	res, err := panel.Tally(reg.Dir)
	if err != nil {
		return err
	}
	statuses, err := panelSlotStatuses(root, res)
	if err != nil {
		return err
	}
	for _, status := range statuses {
		if status.Slot == slot {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", termsafe.Escape(slot), status.Status)
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), panel.RuleLine(res))
	return nil
}

func init() {
	panelSubmitCmd.Flags().String("file", "", "Verdict file; - reads stdin")
	panelSubmitCmd.Flags().String("slot", "", "Roster slot")
	panelSubmitCmd.Flags().Int("round", 0, "Panel round")
	panelSubmitCmd.Flags().String("spec", "", "Owning spec")
	panelCmd.AddCommand(panelSubmitCmd)
}
