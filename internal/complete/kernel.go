package complete

import (
	"errors"
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
		if closed && !kernel.RawClose(s, item) {
			return fmt.Errorf("closed work item %s has not finished its gate\nrecovery: bd reopen %s", idrender.Bead(item), idrender.Bead(item))
		}
		result, err := kernel.PassExcept(kernel.Rules(), s, apply, item)
		if err != nil {
			return err
		}
		s = result.State
		return errors.Join(kernel.CheckWorkItem(s, item), kernel.CheckPassedGate(s, kernel.GateBase(item)))
	})
	return s, err
}

func finishKernelComplete(s kernel.State, item string) error {
	return phase.WithPhaseLock(s.Root, func() error {
		fresh, err := kernel.ReadState(s.Store, s.SpecID, s.SpecDir)
		if err != nil {
			return err
		}
		if fresh.Records[item].Status != "closed" || !fresh.Merged[item] {
			return fmt.Errorf("work item %s has not landed", idrender.Bead(item))
		}
		if err := errors.Join(kernel.CheckHalt(fresh), kernel.CheckPassedGate(fresh, kernel.GateBase(item))); err != nil {
			return err
		}
		accepted, err := kernel.AcceptedSubject(fresh, kernel.GateBase(item))
		if err != nil {
			return err
		}
		if fresh.Records[item].Metadata[kernel.KeyCompletion] != accepted {
			if err = fresh.Store.WriteMetadata(item, map[string]any{kernel.KeyCompletion: accepted}); err != nil {
				return err
			}
		}
		fresh, err = kernel.ReadState(s.Store, s.SpecID, s.SpecDir)
		if err != nil {
			return err
		}
		_, err = kernel.Pass(kernel.Rules(), fresh, true)
		return err
	})
}

func recheckKernelComplete(s kernel.State, item string) error {
	return phase.WithPhaseLock(s.Root, func() error {
		fresh, err := kernel.ReadState(s.Store, s.SpecID, s.SpecDir)
		if err != nil {
			return err
		}
		return errors.Join(kernel.CheckHalt(fresh), kernel.CheckWorkItem(fresh, item), kernel.CheckPassedGate(fresh, kernel.GateBase(item)))
	})
}
