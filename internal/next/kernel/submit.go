package kernel

import "fmt"

type SubmitOptions struct {
	Step     string
	Lease    string
	Artifact string
	Commit   string
	Answer   string
}
type ArtifactCheck func(State, Record, Receipt) error

var artifactChecks = map[string]ArtifactCheck{}

func RegisterArtifactCheck(kind string, check ArtifactCheck) error {
	switch kind {
	case "implement", "review-slot", "fix", "change-judge":
	default:
		return fmt.Errorf("unknown artifact kind %q", kind)
	}
	if _, ok := artifactChecks[kind]; ok {
		return fmt.Errorf("duplicate artifact check %q", kind)
	}
	if check == nil {
		return fmt.Errorf("nil artifact check")
	}
	artifactChecks[kind] = check
	return nil
}
func Submit(state State, options SubmitOptions) (Output, error) {
	if !state.Enrolled {
		return Output{}, ErrPreKernel
	}
	return Output{}, ErrNotBuilt
}
