package complete

import (
	"strings"
	"testing"
)

func TestS134Complete_CheckOutputScope(t *testing.T) {
	out := FormatResult(&Result{Check: true})
	if !strings.Contains(out, "Panel and kernel gates were not checked.") {
		t.Fatalf("content check overstates gate coverage: %s", out)
	}
}
