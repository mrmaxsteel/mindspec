package main

import "os"

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Spec 084 Hard Constraint #5: mindspec otel setup/status need
		// exits 0/1/2 distinctly. otelExitCode unwraps the
		// otel.go-defined error types; default for non-otel errors is
		// still 1 (cobra convention).
		os.Exit(otelExitCode(err))
	}
}
