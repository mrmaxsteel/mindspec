package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/mrmaxsteel/mindspec/internal/config"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/spf13/cobra"
)

var submitCommandStateFn = kernelCommandState

var submitCmd = &cobra.Command{SilenceUsage: true, Use: "submit <step-or-decision>", Short: "Submit evidence or answer a kernel decision", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
	state, err := submitCommandStateFn("", args[0])
	if err != nil {
		return err
	}
	options := kernel.SubmitOptions{Step: args[0]}
	options.Lease, _ = cmd.Flags().GetString("lease")
	options.Artifact, _ = cmd.Flags().GetString("artifact")
	if options.Artifact != "" {
		options.Artifact, err = filepath.Abs(options.Artifact)
		if err != nil {
			return err
		}
	}
	options.Commit, _ = cmd.Flags().GetString("commit")
	options.Answer, _ = cmd.Flags().GetString("answer")
	var out kernel.Output
	if options.Answer != "" {
		out, err = kernel.Answer(state, options.Step, options.Answer)
	} else {
		options.BeforeClose = func(fresh kernel.State, receipt kernel.Receipt) error {
			return publishSubmittedReview(fresh, options.Step, receipt)
		}
		out, err = kernel.Submit(state, options)
	}
	if err != nil {
		return err
	}
	if err = json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
		return err
	}
	if out.Status == "halted" {
		fresh, err := kernel.ReadState(state.Store, state.SpecID, state.SpecDir)
		if err != nil {
			return err
		}
		var halts []string
		for id, answer := range fresh.Answers() {
			if answer.Option == "halt" {
				halts = append(halts, id)
			}
		}
		sort.Strings(halts)
		if len(halts) == 0 {
			return exitErr(4, fmt.Errorf("spec is halted\nrecovery: mindspec next-action"))
		}
		return exitErr(4, fmt.Errorf("spec is halted by %s\nrecovery: mindspec next-action", termsafe.Escape(halts[0])))
	}
	return nil
}}

func init() {
	submitCmd.Flags().String("lease", "", "Current lease token")
	submitCmd.Flags().String("artifact", "", "Artifact path held by a commit")
	submitCmd.Flags().String("commit", "", "Commit that holds the implementation or fix")
	submitCmd.Flags().String("answer", "", "Listed option for a decision")
	submitCmd.MarkFlagsMutuallyExclusive("artifact", "commit", "answer")
	rootCmd.AddCommand(submitCmd)
}

// publishSubmittedReview makes a committed slot verdict visible to the panel tally.
// The kernel invokes this only after it validates the token and artifact.
func publishSubmittedReview(state kernel.State, step string, receipt kernel.Receipt) error {
	row := state.Records[step]
	tuple, _, err := row.Tuple()
	if err != nil || tuple.Kind != "review-slot" {
		return err
	}
	for _, reg := range state.Panels {
		if reg.Err != nil {
			return reg.Err
		}
		if reg.Panel.KernelGate == "" || state.Records[reg.Panel.KernelGate].ID != row.Parent {
			continue
		}
		data, exists, err := newExecutor(state.Root).FileAtRefOrAbsent(receipt.Commit, receipt.Path)
		if err != nil {
			return err
		}
		if !exists {
			return panelVerbFailure(fmt.Errorf("committed verdict is missing"))
		}
		cfg, err := config.Load(state.Root)
		if err != nil {
			return err
		}
		iso, err := config.IsolationRootFor(state.Root)
		if err != nil {
			return err
		}
		slot := fmt.Sprintf("R%d", tuple.Slot)
		layout, err := panel.NewSlotLayout(state.Root, iso, state.SpecID, reg.Slug(), tuple.Round, slot)
		if err != nil {
			return err
		}
		_, err = panel.Submit(panel.SubmitTarget{Dir: reg.Dir, Spec: state.SpecID, Panel: reg.Slug(), Slot: slot, Round: tuple.Round, SlotDir: layout.Dir}, data, func(p panel.Panel, roster panel.RosterSlot) (panel.SubmitResolution, error) {
			resolved, err := resolveEffectivePanelSlot(cfg, reg.Dir, p, roster)
			if err != nil {
				return resolved, err
			}
			item := ""
			if p.BeadID != nil {
				item = *p.BeadID
			}
			resolved.Landed, err = landedBindingFor(panelSubmitMetadataFn, item, reg.Slug(), tuple.Round)
			return resolved, err
		})
		return err
	}
	return panelVerbFailure(fmt.Errorf("slot has no registered panel"))
}
