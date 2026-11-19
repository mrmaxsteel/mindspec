package kerneltest

import "testing"

func TestS134Probe_PinnedVersion(t *testing.T) {
	for output, want := range map[string]bool{"bd version 1.3.4": true, "bd version 1.3.4 (release)": true, "bd version 1.3.40": false, "bd version 1.3.3": false, "bd version 1.3.4-dev": false} {
		if got := pinnedVersion(output); got != want {
			t.Errorf("version %q accepted=%t want=%t", output, got, want)
		}
	}
}
