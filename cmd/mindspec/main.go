package main

import "os"

func main() {
	// Snapshot provenance BEFORE Execute() runs — see
	// sourceRegisteredCommands' doc comment (cmdtree_dump.go): every
	// package init() has already added cmd/mindspec's own commands by
	// this point, and cobra's own auto-registrations
	// (InitDefaultHelpCmd/InitDefaultCompletionCmd) happen only inside
	// Execute(), so this call must come first.
	snapshotSourceRegisteredCommands(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		// Spec 084 Hard Constraint #5: mindspec otel setup/status need
		// exits 0/1/2 distinctly. otelExitCode unwraps the
		// otel.go-defined error types; default for non-otel errors is
		// still 1 (cobra convention).
		os.Exit(otelExitCode(err))
	}
}
