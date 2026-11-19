package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
	"github.com/spf13/cobra"
)

var nextActionCmd = &cobra.Command{
	Use: "next-action [spec]", Short: "Show the next kernel actions", Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spec := ""
		if len(args) > 0 {
			spec = args[0]
		}
		state, err := kernelCommandState(spec, "")
		if err != nil {
			return err
		}
		apply, _ := cmd.Flags().GetBool("apply")
		out, err := kernel.NextAction(state, apply)
		if err != nil {
			return err
		}
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(out); err != nil {
			return err
		}
		if code := kernel.OutputExitCode(out); code != 0 {
			return exitErr(code, fmt.Errorf("next action: %s", out.Status))
		}
		return nil
	},
}

func init() {
	nextActionCmd.Flags().Bool("apply", false, "Apply one reduction under the phase lock")
	nextActionCmd.Flags().Bool("json", false, "Print the action document as JSON")
	rootCmd.AddCommand(nextActionCmd)
}

// kernelCommandState resolves an epic without migration or lifecycle writes.
func kernelCommandState(spec, target string) (kernel.State, error) {
	root, err := workspace.FindRoot(".")
	if err != nil {
		return kernel.State{}, err
	}
	store := kernel.NewStore(root, "", nil)
	records, err := store.All()
	if err != nil {
		return kernel.State{}, fmt.Errorf("read store: %s\nrecovery: mindspec state show", termsafe.Escape(err.Error()))
	}
	epicID := ""
	if target != "" {
		if tuple, e := kernelid.Parse(target); e == nil {
			epicID = tuple.Epic
		} else {
			for _, r := range records {
				if r.ID == target || r.Key() == target {
					tuple, ok, e := r.Tuple()
					if e == nil && ok {
						epicID = tuple.Epic
					}
				}
			}
		}
		if epicID == "" {
			return kernel.State{}, fmt.Errorf("step is not a kernel identity\nrecovery: use mindspec next, mindspec complete and mindspec impl approve")
		}
	}
	candidates := []kernel.Record{}
	for _, r := range records {
		if r.Type != "epic" || (epicID != "" && r.ID != epicID) {
			continue
		}
		id, e := phase.ResolveSpec(phase.EpicInfo{ID: r.ID, Title: r.Title, Metadata: r.Metadata})
		if e != nil {
			continue
		}
		if spec != "" && id != spec && !strings.HasPrefix(id, spec+"-") && !strings.HasSuffix(id, "-"+spec) {
			continue
		}
		if spec == "" && target == "" && r.Status == "closed" {
			continue
		}
		candidates = append(candidates, r)
	}
	if len(candidates) != 1 {
		return kernel.State{}, fmt.Errorf("select one active spec\nrecovery: mindspec state show")
	}
	epic := candidates[0]
	spec, err = phase.ResolveSpec(phase.EpicInfo{ID: epic.ID, Title: epic.Title, Metadata: epic.Metadata})
	if err != nil {
		return kernel.State{}, err
	}
	dir, err := workspace.SpecDir(root, spec)
	if err != nil {
		return kernel.State{}, err
	}
	store.Epic = epic.ID
	state, err := kernel.ReadState(store, spec, dir)
	if err != nil {
		var failure *termsafe.Failure
		if errors.As(err, &failure) {
			return state, termsafe.PrefixFailure(err, "read kernel state: ")
		}
		return state, fmt.Errorf("read kernel state: %s\nrecovery: mindspec state show", termsafe.Escape(err.Error()))
	}
	return state, nil
}
