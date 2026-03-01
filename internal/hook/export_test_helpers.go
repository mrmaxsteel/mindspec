package hook

// ExportGetCwd returns the current getCwd function for test backup.
func ExportGetCwd() func() (string, error) { return getCwd }

// SetGetCwd replaces getCwd for testing from external packages.
func SetGetCwd(fn func() (string, error)) { getCwd = fn }
