package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var nextKernelStatesFn = readNextKernelStates
var nextKernelOrphansFn = checkUnmergedBeads

func readNextKernelStates(root, spec string) ([]kernel.State, error) {
	if _, err := os.Stat(filepath.Join(root, ".beads")); os.IsNotExist(err) && os.Getenv("BEADS_DIR") == "" {
		return nil, nil
	}
	store := kernel.NewStore(root, "", nil)
	rows, err := store.All()
	if err != nil {
		return nil, err
	}
	var states []kernel.State
	for _, r := range rows {
		if r.Type != "epic" || (spec == "" && r.Status == "closed") {
			continue
		}
		id, err := phase.ResolveSpec(phase.EpicInfo{ID: r.ID, Title: r.Title, Metadata: r.Metadata})
		if err != nil || (spec != "" && id != spec && !strings.HasPrefix(id, spec+"-")) {
			continue
		}
		dir, err := workspace.SpecDir(root, id)
		if err != nil {
			return nil, err
		}
		s, err := kernel.ReadState(kernel.NewStore(root, r.ID, nil), id, dir)
		if err != nil {
			return nil, err
		}
		if s.Enrolled {
			states = append(states, s)
		}
	}
	return states, nil
}

func runKernelNext(cmd *cobra.Command, root, spec, name string, pick int, signals []string, flagGiven bool, evidence string) (bool, error) {
	states, err := nextKernelStatesFn(root, spec)
	if err != nil {
		return true, err
	}
	var candidates []kernel.State
	var halted []error
	for _, s := range states {
		matches := name == ""
		n, _ := strconv.Atoi(name)
		for i, item := range s.Items {
			matches = matches || name == item || strings.HasSuffix(item, "-"+name) || n == i+1
		}
		if !matches {
			continue
		}
		if s.Halted() && spec == "" && name == "" {
			halted = append(halted, kernel.CheckHalt(s))
			continue
		}
		candidates = append(candidates, s)
	}
	if len(candidates) == 0 {
		if len(halted) > 0 {
			return true, errors.Join(halted...)
		}
		return false, nil
	}
	if len(candidates) != 1 {
		return true, fmt.Errorf("select one active spec\nrecovery: mindspec next --spec <spec>")
	}
	s := candidates[0]
	if err := kernel.CheckHalt(s); err != nil {
		return true, err
	}
	if err := nextKernelOrphansFn(s.SpecID); err != nil {
		return true, err
	}
	var items []string
	for _, item := range s.Items {
		if s.Records[item].Status != "closed" {
			items = append(items, item)
		}
	}
	selected := ""
	waiting := ""
	if name != "" {
		n, _ := strconv.Atoi(name)
		for i, item := range s.Items {
			if name == item || strings.HasSuffix(item, "-"+name) || n == i+1 {
				selected = item
				break
			}
		}
	} else {
		for _, item := range items {
			step := kernel.ImplementationStep(s, item)
			if step != "" && s.Records[step].Status != "closed" {
				lease, _, err := kernel.Decode[kernel.LeaseRecord](s.Records[step].Metadata, kernel.KeyLease)
				if err != nil {
					return true, err
				}
				if lease.TokenHash != "" && lease.Expires.After(s.At) {
					if waiting == "" {
						waiting = item
					}
					continue
				}
				if kernel.CheckWorkItem(s, step) != nil {
					continue
				}
				selected = item
				break
			}
		}
		if pick > 0 && pick <= len(items) {
			selected = items[pick-1]
		} else if pick > len(items) {
			return true, fmt.Errorf("pick %d out of range", pick)
		}
	}
	if selected == "" && waiting != "" {
		selected = waiting
	}
	if selected == "" {
		return true, fmt.Errorf("no implementation is ready\nrecovery: mindspec next-action %s --apply", idrender.Spec(s.SpecID))
	}
	dirt, ignored, err := next.CheckDirtyTreeProportional(root, root, s.SpecID)
	if err != nil {
		return true, err
	}
	if len(dirt) > 0 {
		return true, next.DirtyTreeFailure(root, dirt, "")
	}
	if len(ignored) > 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), next.IgnoredDirtNotice(ignored))
	}
	step := kernel.ImplementationStep(s, selected)
	if step == "" {
		if _, err := kernel.NextAction(s, true); err != nil {
			return true, err
		}
		s, err = kernel.ReadState(s.Store, s.SpecID, s.SpecDir)
		if err != nil {
			return true, err
		}
		step = kernel.ImplementationStep(s, selected)
	}
	if step == "" {
		return true, fmt.Errorf("implementation is blocked\nrecovery: mindspec next-action %s", idrender.Spec(s.SpecID))
	}
	gate, err := nextGateReadinessFn(root, selected, next.GateOptions{ScopeSpec: s.SpecID, NamedSignals: signals, FlagGiven: flagGiven, EvidencePath: evidence})
	if err != nil {
		return true, err
	}
	if len(gate.FailingSignals) > 0 {
		if err := next.RecordReadinessOverride(selected, gate); err != nil {
			return true, err
		}
	}
	result, err := kernel.Lease(s, kernel.LeaseOptions{Step: step}, kernel.LeaseDeps{EnsureWorktree: func(root, item, spec string) (string, error) {
		return nextEnsureWorktreeFn(root, item, spec, newExecutor(root))
	}})
	if err != nil {
		return true, err
	}
	for _, warning := range result.Warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), warning)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "generation %d\n%s\n", result.Lease.Generation, result.Token)
	return true, err
}
