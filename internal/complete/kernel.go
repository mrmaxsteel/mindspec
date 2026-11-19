package complete

import (
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/idvalidate/idrender"
	"github.com/mrmaxsteel/mindspec/internal/next/kernel"
	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/workspace"
)

var completeKernelStateFn = func(root, spec, epic string) (*kernel.State, error) {
	if epic == "" {
		return nil, nil
	}
	dir, err := workspace.SpecDir(root, spec)
	if err != nil {
		return nil, err
	}
	store := kernel.NewStore(root, epic, runBDFn)
	rows, err := store.All()
	if err != nil {
		return nil, err
	}
	exists := false
	for _, r := range rows {
		exists = exists || r.ID == epic
	}
	if !exists {
		return nil, nil
	}
	s, err := kernel.ReadState(store, spec, dir)
	if err != nil {
		return nil, err
	}
	if !s.Enrolled {
		return nil, nil
	}
	return &s, nil
}

func prepareKernelComplete(s kernel.State, item string, apply bool) (kernel.State, error) {
	if err := kernel.CheckHalt(s); err != nil {
		return s, err
	}
	err := phase.WithPhaseLock(s.Root, func() error {
		fresh, err := kernel.ReadState(s.Store, s.SpecID, s.SpecDir)
		if err != nil {
			return err
		}
		s = fresh
		if err := kernel.CheckHalt(s); err != nil {
			return err
		}
		found := false
		for _, id := range s.Items {
			found = found || id == item
		}
		if !found {
			return fmt.Errorf("work item %s is not in the approved plan", idrender.Bead(item))
		}
		closed := s.Records[item].Status == "closed"
		if closed && kernel.ForcedClose(s, item) {
			return fmt.Errorf("closed work item %s has not finished its gate\nrecovery: bd reopen %s", idrender.Bead(item), idrender.Bead(item))
		}
		if closed && s.Merged[item] {
			return kernel.CheckPassedGate(s, kernel.GateBase(item))
		}
		if err := kernel.CheckWorkItem(s, item); err != nil {
			return err
		}
		if closed && !kernel.RawClose(s, item) {
			return fmt.Errorf("closed work item %s has not finished its gate\nrecovery: bd reopen %s", idrender.Bead(item), idrender.Bead(item))
		}
		result, err := kernel.PassExcept(kernel.Rules(), s, apply, item)
		if err != nil {
			return err
		}
		s = result.State
		return kernel.CheckPassedGate(s, kernel.GateBase(item))
	})
	return s, err
}

func finishKernelComplete(s kernel.State) error {
	return phase.WithPhaseLock(s.Root, func() error {
		fresh, err := kernel.ReadState(s.Store, s.SpecID, s.SpecDir)
		if err != nil {
			return err
		}
		_, err = kernel.Pass(kernel.Rules(), fresh, true)
		return err
	})
}
