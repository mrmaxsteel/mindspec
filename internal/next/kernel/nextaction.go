package kernel

// NextAction is the reducer entry point.
func NextAction(state State, apply bool) (Output, error) {
	if !state.Enrolled {
		return Output{}, ErrPreKernel
	}
	return Output{}, ErrNotBuilt
}
