package kernel

import (
	"errors"
	"fmt"

	"github.com/mrmaxsteel/mindspec/internal/next/kernel/kernelid"
)

// Pass dispatches the fixed rule order and overlays every write for later rules.
func Pass(reg *Registry, state State, apply bool) (PassResult, error) {
	return passExcept(reg, state, apply, "")
}

// passExcept omits the named step from target lists and blocks writes to it.
func passExcept(reg *Registry, state State, apply bool, step string) (PassResult, error) {
	var result PassResult
	if !state.Enrolled {
		return result, ErrPreKernel
	}
	if apply && state.Store == nil {
		return result, fmt.Errorf("apply needs a store")
	}
	if apply {
		// Cleanup is best effort. Each metadata write checks its directory again.
		_ = SweepMetadata(state.Root)
	}
	err := WithCASRetry(func() error {
		var err error
		result, err = reduce(reg, state, apply, step)
		if err != nil {
			return err
		}
		if !apply {
			return nil
		}
		for _, write := range result.Writes {
			if err = applyWrite(state.Store, write); err != nil {
				if errors.Is(err, ErrCASMismatch) {
					fresh, e := ReadState(state.Store, state.SpecID, state.SpecDir)
					if e != nil {
						return e
					}
					state = fresh
				}
				return err
			}
		}
		return nil
	})
	return result, err
}
func reduce(reg *Registry, state State, apply bool, step string) (PassResult, error) {
	result := PassResult{State: state.Clone(), Output: Output{Schema: "mindspec.next-action/v2", Status: "wait", Applied: apply, Policy: PolicyView{RoundLimit: state.Policy.RoundLimit, LeaseTTL: state.Policy.LeaseTTL}, Actions: []Action{}, Waiting: []Waiting{}, Decisions: []Decision{}, Run: []Run{}, Flags: []Flag{}}, Writes: []Write{}}
	for _, name := range reg.names {
		entry, ok := reg.entries[name]
		if !ok {
			continue
		}
		halted := result.State.Halted()
		if halted && entry.mode == Ordinary {
			continue
		}
		input := result.State.Clone()
		excluded := map[string]bool{step: step != ""}
		for key, record := range input.Records {
			if step != "" && (key == step || record.ID == step) {
				excluded[key], excluded[record.ID] = true, true
				excludedRecord := record
				input.ExcludedStep = &excludedRecord
				tuple, has, err := record.Tuple()
				if err != nil || !has || (tuple.Kind != "bead-gate" && tuple.Kind != "final-gate") {
					delete(input.Records, key)
				}
				break
			}
		}
		input.classify()
		changes, err := entry.fn(input)
		if err != nil {
			return result, fmt.Errorf("rule %s: %w", name, err)
		}
		if entry.mode == FlagOnly && len(changes.Writes) > 0 {
			return result, fmt.Errorf("flag rule %s returned writes", name)
		}
		for _, write := range changes.Writes {
			if excluded[write.ID] {
				continue
			}
			if halted && (write.Kind != "close" || write.Receipt == nil || write.Receipt.Kind != "halted") {
				return result, fmt.Errorf("halt rule may only finish halted closes")
			}
			if err = overlay(&result.State, write); err != nil {
				return result, err
			}
			result.Writes = append(result.Writes, write)
		}
		result.Output.Flags = append(result.Output.Flags, changes.Flags...)
		result.Output.Actions = append(result.Output.Actions, changes.Actions...)
		result.Output.Waiting = append(result.Output.Waiting, changes.Waiting...)
		result.Output.Decisions = append(result.Output.Decisions, changes.Decisions...)
		result.Output.Run = append(result.Output.Run, changes.Run...)
	}
	if len(result.Output.Actions)+len(result.Output.Decisions)+len(result.Output.Run) > 0 {
		result.Output.Status = "work"
	}
	if result.State.Halted() {
		result.Output.Status = "halted"
	}
	return result, nil
}
func overlay(state *State, w Write) error {
	switch w.Kind {
	case "create":
		if w.Create == nil || w.ID != w.Create.ID {
			return fmt.Errorf("create needs a matching identity")
		}
		t, err := kernelid.Parse(w.ID)
		if err != nil || t != w.Create.Tuple || t.Epic != state.Epic {
			return fmt.Errorf("create needs a matching tuple")
		}
		if _, exists := state.Records[w.ID]; exists {
			return fmt.Errorf("id-collision: create already exists")
		}
		if w.Create.Type != "step" && w.Create.Type != "decision" {
			return fmt.Errorf("kernel type must be step or decision")
		}
		if (t.Kind == "decision") != (w.Create.Type == "decision") {
			return fmt.Errorf("type does not match kernel kind")
		}
		metadata := map[string]any{}
		for k, v := range w.Create.Metadata {
			metadata[k] = v
		}
		metadata[KeyID] = w.ID
		metadata[KeyTuple] = w.Create.Tuple
		state.Records[w.ID] = Record{ID: w.ID, Parent: w.Create.Parent, Type: w.Create.Type, Title: markedTitle(w.Create.Title), Labels: []string{"mindspec"}, Status: "open", Metadata: metadata}
	case "metadata", "close", "link":
		r, ok := state.Records[w.ID]
		if !ok {
			return fmt.Errorf("write target is absent")
		}
		owned := w.ID == state.Epic
		for _, item := range state.Items {
			owned = owned || w.ID == item
		}
		if tuple, err := kernelid.Parse(w.ID); err == nil {
			have, ok, e := r.Tuple()
			owned = owned || (e == nil && ok && tuple == have && tuple.Epic == state.Epic)
		}
		if !owned {
			return fmt.Errorf("foreign write target")
		}
		if r.Metadata == nil {
			r.Metadata = map[string]any{}
		}
		switch w.Kind {
		case "metadata":
			for k, v := range w.Metadata {
				r.Metadata[k] = v
			}
		case "close":
			if w.Receipt == nil {
				return fmt.Errorf("close needs a receipt")
			}
			if err := ValidReceipt(state.Root, *w.Receipt, r.Metadata); err != nil {
				return err
			}
			r.Metadata[KeyClose] = *w.Receipt
			r.Status = "closed"
		case "link":
			if _, ok := state.Records[w.Upstream]; !ok {
				return fmt.Errorf("link target is absent")
			}
			r.Dependencies = append(r.Dependencies, Dependency{ID: w.Upstream, Type: "blocks"})
		}
		state.Records[w.ID] = r
	default:
		return fmt.Errorf("unknown write kind %q", w.Kind)
	}
	state.classify()
	return nil
}
func applyWrite(s *Store, w Write) error {
	switch w.Kind {
	case "create":
		_, err := s.Create(*w.Create)
		return err
	case "metadata":
		return s.WriteMetadata(w.ID, w.Metadata)
	case "link":
		return s.AddLink(w.ID, w.Upstream)
	case "close":
		r, err := s.owned(w.ID)
		if err != nil {
			return err
		}
		if err = s.WriteReceipt(w.ID, *w.Receipt); err != nil {
			return err
		}
		if r.Status == "closed" {
			return nil
		}
		return s.SetStatusIf(w.ID, r.Status, "closed")
	}
	return fmt.Errorf("unknown write kind")
}

// PassExcept reduces other steps before a verb checks its named step.
func PassExcept(reg *Registry, state State, apply bool, step string) (PassResult, error) {
	return passExcept(reg, state, apply, step)
}
