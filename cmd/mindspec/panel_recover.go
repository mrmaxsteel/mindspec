package main

import (
	"fmt"
	"path/filepath"

	"github.com/mrmaxsteel/mindspec/internal/panel"
	"github.com/mrmaxsteel/mindspec/internal/panellaunch"
	"github.com/spf13/cobra"
)

var panelRecoverCmd = &cobra.Command{
	Use: "recover <slug> <slot> --reason R", Short: "Submit the recorded output of a process attempt",
	Args: cobra.ExactArgs(2), SilenceUsage: true, RunE: runPanelRecover,
}

func runPanelRecover(cmd *cobra.Command, args []string) (runErr error) {
	defer func() { runErr = checkoutError(runErr, "check the attempt output, then repeat panel recover") }()
	reason, _ := cmd.Flags().GetString("reason")
	if reason == "" {
		return checkoutFailure("a recovery reason is required", "set --reason to explain the recovery")
	}
	root, reg, layout, err := launchTarget(cmd, args)
	if err != nil {
		return err
	}
	cfg, _, _, err := launchResolution(root, reg, args[1], true, false)
	if err != nil {
		return err
	}
	if err := checkCheckoutPath(root, filepath.Dir(layout.Lock)); err != nil {
		return err
	}
	lock, err := panel.AcquireSlotLock(layout.Lock)
	if err != nil {
		return err
	}
	defer lock.Release()
	events, err := panel.SlotAttempts(reg.Dir, reg.Panel.Round)
	if err != nil {
		return err
	}
	var start, outcome panel.LaunchRecord
	for _, event := range events[args[1]] {
		if event.Status == "started" && event.Attempt > start.Attempt {
			start = event.LaunchRecord
		}
	}
	if start.Attempt == 0 {
		return checkoutFailure("slot has no process attempt", "mindspec panel launch "+reg.Slug()+" "+args[1])
	}
	for _, event := range events[args[1]] {
		if event.Attempt == start.Attempt && event.Status != "started" {
			outcome = event.LaunchRecord
		}
	}
	if outcome.Status == "" {
		return checkoutFailure("latest attempt has no outcome", "mindspec panel launch "+reg.Slug()+" "+args[1])
	}
	dir := layout.AttemptDir(start.Attempt)
	if err := checkCheckoutPath(root, dir); err != nil {
		return err
	}
	digests, err := panellaunch.Digests(dir)
	if err != nil {
		return err
	}
	for name, want := range map[string]string{"stdout.log": outcome.StdoutSHA256, "stderr.log": outcome.StderrSHA256, "verdict.json": outcome.VerdictSHA256} {
		if digests[name] != want {
			return checkoutFailure("attempt output digest differs: "+name, "use the unchanged output from the latest attempt")
		}
	}
	runner, ok := cfg.RunnerFor(start.Family)
	if !ok {
		return checkoutFailure("attempt runner is no longer configured", "restore the runner in panel.runners")
	}
	data, err := panellaunch.Verdict(dir, runner.Output, start.NonceSHA256)
	if err != nil {
		return err
	}
	selected, err := launchSubmit(root, reg, args[1], start.Attempt, data, "recover", reason)
	if err != nil {
		return err
	}
	outcome.Status, outcome.Seq = "submitted", selected.Seq
	if err := appendLaunchRecord(reg.Dir, outcome); err != nil {
		return err
	}
	exit := "unknown"
	if outcome.ExitCode != nil {
		exit = fmt.Sprint(*outcome.ExitCode)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "attempt %d exit %s seq %d\n", start.Attempt, exit, selected.Seq)
	return launchStatus(cmd, root, reg, args[1])
}

func init() {
	panelRecoverCmd.Flags().String("spec", "", "Select the owning spec")
	panelRecoverCmd.Flags().String("reason", "", "Reason for submitting the recorded output")
	panelCmd.AddCommand(panelRecoverCmd)
}
