package main

import (
	"strings"
	"testing"
)

// The artifact help names what a review slot accepts: its attempt verdict
// file or a file in the registered panel directory. It does not offer a
// committed file to a slot.
func TestSubmitArtifactHelpNamesSlotFiles(t *testing.T) {
	usage := submitCmd.Flags().Lookup("artifact").Usage
	for _, want := range []string{"attempt verdict file", "registered panel directory", "Other steps take a file held by a commit"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("artifact help lacks %q: %s", want, usage)
		}
	}
}
