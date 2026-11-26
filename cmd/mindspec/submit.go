package main

import (
	"encoding/json"
	"fmt"
	"os"
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
	cmd.SilenceErrors = false
	state, err := submitCommandStateFn("", args[0])
	if err != nil {
		return err
	}
	options := kernel.SubmitOptions{Step: args[0]}
	options.Lease, _ = cmd.Flags().GetString("lease")
	options.Repair, _ = cmd.Flags().GetBool("repair")
	options.Reason, _ = cmd.Flags().GetString("reason")
	options.Record, _ = cmd.Flags().GetString("record")
	options.Artifact, _ = cmd.Flags().GetString("artifact")
	for _, path := range []*string{&options.Record, &options.Artifact} {
		if *path != "" && !filepath.IsAbs(*path) {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			// The kernel checks the supplied spelling before it cleans the path.
			*path = cwd + string(filepath.Separator) + *path
		}
	}
	options.Commit, _ = cmd.Flags().GetString("commit")
	options.Answer, _ = cmd.Flags().GetString("answer")
	if options.Answer == "" {
		options.BeforeClose = func(fresh kernel.State, receipt kernel.Receipt) error {
			return publishSubmittedReview(fresh, options.Step, receipt)
		}
	}
	out, err := kernel.Submit(state, options)
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
		message := "spec is halted"
		if len(halts) > 0 {
			message += " by " + termsafe.Escape(halts[0])
		}
		fmt.Fprintln(cmd.ErrOrStderr(), message+"\nrecovery: mindspec next-action")
		cmd.SilenceErrors = true
		return exitErr(4, fmt.Errorf("%s", message))
	}
	return nil
}}

func init() {
	submitCmd.Flags().String("lease", "", "Current lease token")
	submitCmd.Flags().String("record", "", "Record file outside this repository")
	submitCmd.Flags().String("artifact", "", "Artifact path. A review slot takes its attempt verdict file or a file in the registered panel directory of its gate (with no registered panel, a file in a worktree of this repository). Other steps take a file held by a commit.")
	submitCmd.Flags().String("commit", "", "Commit that holds the implementation or fix")
	submitCmd.Flags().String("answer", "", "Listed option for a decision")
	submitCmd.Flags().Bool("repair", false, "Receipt an unreceipted closed step with fitting evidence")
	submitCmd.Flags().String("reason", "", "Reason for repairing a closed step")
	submitCmd.MarkFlagsMutuallyExclusive("record", "artifact", "commit", "answer")
	rootCmd.AddCommand(submitCmd)
}

// publishSubmittedReview writes a stored slot verdict to the registered panel
// directory, where the panel tally reads it. The kernel invokes this only
// after it validates the token and the verdict.
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
		if reg.Panel.KernelGate == "" || !panelOwnsSlotParent(state, reg, row.Parent) {
			continue
		}
		data, err := kernel.ReceiptBytes(row, receipt)
		if err != nil {
			return err
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

// panelOwnsSlotParent reports whether the registered panel reg holds the
// verdicts of slots under the gate record parent. A gate retry keeps the
// panel that is registered on an earlier attempt of the same gate, when the
// retry carries the registration of that panel slug.
func panelOwnsSlotParent(state kernel.State, reg panel.Registration, parent string) bool {
	gate := reg.Panel.KernelGate
	if state.Records[gate].ID == parent {
		return true
	}
	active := kernelPanelGate(state, reg.Panel)
	if active == gate || state.Records[active].ID != parent || gateBase(state, active) != gateBase(state, gate) {
		return false
	}
	registration, has, err := kernel.Decode[kernel.RegistrationRecord](state.Records[active].Metadata, kernel.KeyRegistration)
	return err == nil && has && registration.Slug == reg.Slug()
}
