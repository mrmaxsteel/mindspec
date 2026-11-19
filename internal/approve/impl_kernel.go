package approve

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/guard"
	"github.com/mrmaxsteel/mindspec/internal/journal"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
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
	s, err := kernel.ReadState(store, specID, dir)
	if err != nil {
		return nil, err
	}
	if !s.Enrolled {
		return nil, nil
	}
	return &s, nil
}

func implKernelPreflight(s kernel.State) error {
	// Compute refusals first so malformed audit records cannot hide a blocking id.
	flags := kernel.ApprovalBlockers(s)
	events, err := journal.ReadEvents()
	if err != nil {
		fmt.Fprintf(warnWriter, "warning: cannot read local audit journal: %s\n", termsafe.Escape(err.Error()))
	}
	rows, auditErr := kernel.Audit(s, events)
	if auditErr == nil {
		kernel.PrintAudit(warnWriter, s, rows)
	}
	if len(flags) > 0 {
		var reasons []string
		for _, flag := range flags {
			reasons = append(reasons, flag.Code+": "+termsafe.Escape(flag.Step))
		}
		return guard.NewFailure("implementation approval is blocked:\n"+strings.Join(reasons, "\n"), fmt.Sprintf("resolve the listed records, then run mindspec impl approve %s", termsafe.Escape(s.SpecID)))
	}
	if auditErr != nil {
		fmt.Fprintf(warnWriter, "warning: cannot read kernel audit: %s\n", termsafe.Escape(auditErr.Error()))
	}
	return nil
}
