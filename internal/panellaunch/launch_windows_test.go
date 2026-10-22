//go:build windows

package panellaunch

import (
	"strings"
	"testing"
)

func TestPanelLaunch_WindowsRefuses(t *testing.T) {
	for _, err := range []error{Supported(), Run(Request{})} {
		if err == nil || !strings.Contains(err.Error(), "process runners need a Unix host") {
			t.Fatalf("refusal=%v", err)
		}
	}
}
