package kernel

import (
	"fmt"
	"strings"

	"github.com/mrmaxsteel/mindspec/internal/phase"
	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

// NextAction is the reducer entry point.
func NextAction(state State, apply bool) (Output, error) {
	if !state.Enrolled {
		return Output{}, ErrPreKernel
	}
	approved, ok, err := Decode[PlanApproval](state.Records[state.Epic].Metadata, KeyPlanApproval)
	if err != nil || !ok || !bool(approved) {
		return Output{}, fmt.Errorf("the plan is not approved\nrecovery: mindspec plan approve %s", termsafe.Escape(state.SpecID))
	}
	var out Output
	run := func() error {
		if apply {
			fresh, err := ReadState(state.Store, state.SpecID, state.SpecDir)
			if err != nil {
				return err
			}
			state = fresh
		}
		approved, ok, err := Decode[PlanApproval](state.Records[state.Epic].Metadata, KeyPlanApproval)
		if err != nil || !ok || !bool(approved) {
			return fmt.Errorf("the plan is not approved\nrecovery: mindspec plan approve %s", termsafe.Escape(state.SpecID))
		}
		if apply {
			if err := RequireStepType(state.Root); err != nil {
				return err
			}
		}
		// A pass that writes can make work that only a later pass lists,
		// for example a gate retry and then the slot retry under it. An
		// applied pass that wrote and lists nothing runs again, so one
		// --apply pass does not return a wait with nothing to wait for.
		for passes := 1; ; passes++ {
			result, err := Pass(Rules(), state, apply)
			if err != nil {
				return err
			}
			if apply {
				fresh, err := ReadState(state.Store, state.SpecID, state.SpecDir)
				if err != nil {
					return err
				}
				fresh.At = result.State.At
				state = fresh
			}
			out = OutputForPass(result, state, apply)
			if !apply || len(result.Writes) == 0 || passes >= maxApplyPasses || !idle(out) {
				return nil
			}
		}
	}
	if apply {
		if state.Store == nil {
			return out, fmt.Errorf("apply needs a store\nrecovery: mindspec state show")
		}
		if err := RequireStepType(state.Root); err != nil {
			return out, err
		}
		// Check store availability before creating even the lock file.
		if _, err := state.Store.All(); err != nil {
			return out, actionError(state, err)
		}
		err := phase.WithPhaseLock(state.Root, run)
		return out, actionError(state, err)
	}
	err = run()
	return out, actionError(state, err)
}

// maxApplyPasses bounds the passes of one applied next-action call.
const maxApplyPasses = 4

// idle reports an output with nothing to do and nothing to wait for.
func idle(out Output) bool {
	return out.Status == "wait" && len(out.Actions)+len(out.Decisions)+len(out.Run)+len(out.Waiting) == 0
}

func actionError(s State, err error) error {
	if err == nil || strings.Contains(err.Error(), "\nrecovery:") {
		return err
	}
	return fmt.Errorf("%s\nrecovery: mindspec next-action %s --apply", termsafe.Escape(err.Error()), termsafe.Escape(s.SpecID))
}
