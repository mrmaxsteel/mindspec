package main

import "testing"

func TestInitFlags_GreenfieldOnly(t *testing.T) {
	t.Parallel()

	if initCmd.Flags().Lookup("dry-run") == nil {
		t.Fatal("expected --dry-run flag")
	}
	for _, removed := range []string{"brownfield", "apply", "archive", "resume", "json"} {
		if initCmd.Flags().Lookup(removed) != nil {
			t.Fatalf("did not expect legacy migration flag --%s on init", removed)
		}
	}
}
