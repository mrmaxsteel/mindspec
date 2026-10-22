//go:build windows

package panellaunch

func Run(Request) error            { return Supported() }
func StopOrphan(string, int) error { return nil }
