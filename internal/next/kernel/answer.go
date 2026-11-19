package kernel

func Answer(state State, decision, option string) (Output, error) {
	if !state.Enrolled {
		return Output{}, ErrPreKernel
	}
	return Output{}, ErrNotBuilt
}
