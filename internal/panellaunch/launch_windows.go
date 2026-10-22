//go:build windows

package panellaunch

import "fmt"

func Supported() error {
	return fmt.Errorf("process runners need a Unix host\nrecovery: use a harness runner")
}
func Run(Request) error            { return Supported() }
func StopOrphan(string, int) error { return nil }
