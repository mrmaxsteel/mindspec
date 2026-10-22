package panellaunch

import (
	"runtime"

	"github.com/mrmaxsteel/mindspec/internal/termsafe"
)

func Supported() error { return SupportedOn(runtime.GOOS) }

// SupportedOn checks process runner support for the selected host platform.
func SupportedOn(goos string) error {
	if goos == "windows" {
		return termsafe.NewFailure("process runners need a Unix host", "use a harness runner")
	}
	return nil
}
