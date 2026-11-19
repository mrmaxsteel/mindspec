package kernel

func Answer(state State, decision, option string) (Output, error) {
	if !state.Enrolled {
		return Output{}, ErrPreKernel
	}
	var output Output
	err := withActorLock(state, func(s State) error {
		var err error
		output, err = answerLocked(s, decision, option)
		return err
	})
	return output, err
}
