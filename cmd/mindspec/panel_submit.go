package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/bead"
	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/safefile"
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

// findPanelSubmitSlot also accepts saved verdicts in a slot attempt directory.
// Workspace boundaries still stop lookup, including workspaces inside attempts.
func findPanelSubmitSlot(dir string) (workspace.SlotRoot, bool, error) {
	slot, found, err := workspace.FindSlotRoot(dir, config.IsolationRootFor)
	if err != nil || found {
		return slot, found, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return workspace.SlotRoot{}, false, err
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	for {
		if marked, err := workspace.HasRootMarker(dir); err != nil {
			return workspace.SlotRoot{}, false, err
		} else if marked {
			return workspace.SlotRoot{}, false, nil
		}
		name := filepath.Base(dir)
		if n, err := strconv.Atoi(strings.TrimPrefix(name, "attempt-")); err == nil && n > 0 && name == "attempt-"+strconv.Itoa(n) {
			parent := filepath.Dir(dir)
			return workspace.FindSlotRoot(parent, config.IsolationRootFor)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return workspace.SlotRoot{}, false, nil
		}
		dir = parent
	}
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
	fromCWD, hasCWD, err := findPanelSubmitSlot(cwd)
	if err != nil {
		return err
	}
	var fromFile workspace.SlotRoot
	hasFile := false
	if filepath.IsAbs(file) {
		fromFile, hasFile, err = findPanelSubmitSlot(filepath.Dir(file))
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
	rosterIndex := -1
	for i, reviewer := range reg.Panel.Roster {
		if reviewer.Slot == slot {
			rosterIndex = i
			break
		}
	}
	if rosterIndex < 0 {
		return guard.NewFailure("slot is not on the roster", "select a slot from the panel roster")
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
	input := cmd.InOrStdin()
	if file != "-" {
		f, err := safefile.Open(file)
		if err != nil {
			return guard.NewFailure(termsafe.Escape(err.Error()), "select a readable regular verdict file")
		}
		defer f.Close()
		input = f
	}
	data, err := io.ReadAll(io.LimitReader(input, panel.MaxVerdictBytes+1))
	if err != nil {
		return guard.NewFailure(termsafe.Escape(err.Error()), "select a readable verdict file and submit it again")
	}
	target := panel.SubmitTarget{Dir: reg.Dir, Spec: spec, Panel: slug, Slot: slot, Round: round, File: file, SlotDir: layout.Dir}
	submit := func() error {
		_, err := panel.Submit(target, data, func(p panel.Panel, roster panel.RosterSlot) (panel.SubmitResolution, error) {
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
		return err
	}
	var state *kernel.State
	if reg.Panel.Gate == "bead" || reg.Panel.Gate == "final_review" {
		state, err = panelKernelStateFn(root, spec)
		if err != nil {
			return err
		}
	}
	if state != nil && (reg.Panel.Gate == "bead" || reg.Panel.Gate == "final_review") {
		token, _ := cmd.Flags().GetString("lease")
		gate := kernelPanelGate(*state, reg.Panel)
		if slot != fmt.Sprintf("R%d", rosterIndex+1) {
			return guard.NewFailure("slot identity differs from its roster position", "select a slot from the panel roster")
		}
		step := fmt.Sprintf("%s.r%d.R%d", gateBase(*state, gate), round, rosterIndex+1)
		head, e := kernel.SlotLineageHead(*state, step)
		if e != nil {
			return e
		}
		step = head.Key()
		if file == "-" {
			// The kernel closes a slot from a verdict file: the attempt file
			// that the slot prompt names, or a file in the registered panel
			// directory.
			return guard.NewFailure("this slot does not read a verdict from stdin", kernel.SlotPromptRecovery(*state, step))
		}
		if token == "" {
			argv := "mindspec panel submit --file " + shellQuoteTarget(file)
			if !inferred {
				argv = fmt.Sprintf("mindspec panel submit %s --file %s --slot %s --round %d --spec %s", shellQuoteTarget(slug), shellQuoteTarget(file), shellQuoteTarget(slot), round, shellQuoteTarget(spec))
			}
			return guard.NewFailure("--lease is required for this slot", argv+" --lease <token> (the token that mindspec lease "+shellQuoteTarget(step)+" printed)")
		}
		artifact, e := filepath.Abs(file)
		if e != nil {
			return e
		}
		_, err = kernel.Submit(*state, kernel.SubmitOptions{Step: step, Lease: token, Artifact: artifact, BeforeClose: func(fresh kernel.State, receipt kernel.Receipt) error {
			stored, e := kernel.ReceiptBytes(fresh.Records[step], receipt)
			if e != nil {
				return e
			}
			data = stored
			return submit()
		}})
	} else {
		err = submit()
	}
	if err != nil {
		return err
	}
	res, statuses, err := panelTallyStatuses(root, reg.Dir)
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
	panelSubmitCmd.Flags().String("lease", "", "Current slot lease token")
	panelSubmitCmd.Flags().String("file", "", "Verdict file, or - for stdin; a slot of a governed spec takes the file in its attempt directory or the file at its registered panel path, not stdin")
	panelSubmitCmd.Flags().String("slot", "", "Roster slot")
	panelSubmitCmd.Flags().Int("round", 0, "Panel round")
	panelSubmitCmd.Flags().String("spec", "", "Owning spec")
	panelCmd.AddCommand(panelSubmitCmd)
}
