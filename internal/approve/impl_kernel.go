package approve

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/journal"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

var implKernelStateFn = readImplKernelState

func readImplKernelState(root, specID string) (*kernel.State, error) {
	epic, err := phase.FindEpicBySpecID(specID)
	if err != nil {
		return nil, err
	}
	dir, err := workspace.SpecDir(root, specID)
	if err != nil {
		return nil, err
	}
	return implKernelState(kernel.NewStore(root, epic, nil), specID, dir)
}

func implKernelState(store *kernel.Store, specID, dir string) (*kernel.State, error) {
	enrolled, err := implKernelEnrolled(store, dir)
	if err != nil || !enrolled {
		return nil, err
	}
	s, err := kernel.ReadState(store, specID, dir)
	if err != nil {
		return nil, err
	}
	if !s.Enrolled {
		return nil, nil
	}
	return &s, nil
}

// implKernelEnrolled checks the same subtree as ReadState before reading evidence.
// A pre-kernel approval needs no kernel graph or evidence read.
func implKernelEnrolled(store *kernel.Store, dir string) (bool, error) {
	out, err := store.Run("show", store.Epic, "--json")
	if err != nil {
		return false, err
	}
	var records []kernel.Record
	if err := json.Unmarshal(out, &records); err != nil {
		return false, err
	}
	for _, r := range records {
		if r.ID == store.Epic {
			if _, present := r.Metadata[kernel.KeyRun]; present {
				return true, nil
			}
		}
	}
	orchestrated, err := kernelid.PanelOrchestrated(dir)
	if err != nil || orchestrated {
		return orchestrated, err
	}
	all, err := store.All()
	if err != nil {
		return false, err
	}
	subtree := kernel.Subtree(all, store.Epic)
	for _, r := range all {
		if !subtree[r.ID] {
			continue
		}
		tuple, parseErr := kernelid.Parse(r.Key())
		have, ok, tupleErr := r.Tuple()
		if parseErr == nil && tuple.Epic == store.Epic && tupleErr == nil && ok && have == tuple {
			return true, nil
		}
	}
	return false, nil
}

func implKernelPreflight(s kernel.State) error {
	// Compute refusals first so malformed audit records cannot hide a blocking id.
	flags, blockerErr := kernel.ApprovalBlockers(s)
	events, err := journal.ReadEvents()
	if err != nil {
		fmt.Fprintf(warnWriter, "warning: cannot read local audit journal: %s\n", termsafe.Escape(err.Error()))
	}
	rows, auditErr := kernel.Audit(s, events)
	if auditErr == nil {
		kernel.PrintAudit(warnWriter, s, rows)
	}
	refusal := ""
	if blockerErr != nil {
		refusal = "cannot check implementation approval: " + termsafe.Escape(blockerErr.Error())
	} else if len(flags) > 0 {
		var reasons []string
		for _, flag := range flags {
			line := termsafe.Escape(flag.Code) + ": " + termsafe.Escape(flag.Step)
			if len(flag.Repair) > 0 {
				args := make([]string, len(flag.Repair))
				for i, arg := range flag.Repair {
					args[i] = termsafe.Escape(arg)
				}
				line += "; repair: " + strings.Join(args, " ")
			}
			reasons = append(reasons, line)
		}
		refusal = "implementation approval is blocked:\n" + strings.Join(reasons, "\n")
	}
	if refusal != "" {
		return guard.NewFailure(refusal, fmt.Sprintf("resolve the listed records, then run mindspec impl approve %s", idrender.Spec(s.SpecID)))
	}
	if auditErr != nil {
		fmt.Fprintf(warnWriter, "warning: cannot read kernel audit: %s\n", termsafe.Escape(auditErr.Error()))
	}
	return nil
}
