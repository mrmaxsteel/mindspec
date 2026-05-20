package main

// cmd/mindspec/otel.go: the new `mindspec otel setup` / `mindspec otel
// status` cobra commands. Spec 084 (mindspec-otel-only) Bead 1.
//
// Per spec 084 Hard Constraint #5, this file performs ZERO network I/O.
// All endpoint validation is pure URL parsing (url.Parse) which the
// specgate AST check (Bead 4) explicitly allows. No net.Dial, no
// http.Get, no http.Client.Do anywhere in this file or in
// internal/otel/.
//
// Per spec 084 Migration Commits 1-2, the new otel surfaces land FIRST
// in Bead 1 while the legacy `mindspec agentmind setup` remains in
// place. Users can diff `mindspec agentmind setup --endpoint X` output
// against `mindspec otel setup --endpoint X` output for one bead to
// verify equivalence before legacy removal in Bead 3.

import (
	"fmt"
	"os"
	"sort"

	"github.com/mrmaxsteel/mindspec/internal/otel"
	"github.com/spf13/cobra"
)

var otelCmd = &cobra.Command{
	Use:   "otel",
	Short: "Configure OTEL endpoint for downstream workloads",
	Long: `Render the user-supplied OTEL endpoint into the formats workloads consume.

mindspec emits OTEL configuration; it never spawns a collector and
never speaks OTLP itself. Anything that speaks OTLP works:
agentmind, Honeycomb, Tempo, Jaeger, opentelemetry-collector-contrib.

Subcommands:
  setup   Write OTEL endpoint to Claude/Codex config (or print env exports)
  status  Print the currently configured OTEL endpoint (read-only)
`,
}

// otelSetupCmd implements `mindspec otel setup` per spec 084 Hard
// Constraint #5. Writes to one or more of:
//   - <root>/.claude/settings.local.json (Claude Code target; default)
//   - ~/.codex/config.toml (Codex target; --codex or --target=codex)
//   - stdout (env-export form; --target=env)
//
// Exit code matrix (spec Hard Constraint #5):
//
//	0 — config written (or already up-to-date and re-write was idempotent)
//	1 — pre-existing target TOML/JSON failed to parse; no modification
//	2 — invocation error (missing --endpoint, unknown --protocol, etc.)
var otelSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Write OTEL endpoint configuration to Claude/Codex/env",
	Long: `Write the OTEL endpoint to one of three targets:

  claude  .claude/settings.local.json in the current project (default)
  codex   ~/.codex/config.toml (or --codex-config <path>)
  env     stdout, as POSIX-shell 'export KEY=VALUE' lines

Examples:

  mindspec otel setup --endpoint http://collector.example:4318
  mindspec otel setup --endpoint http://localhost:4318 --codex
  mindspec otel setup --endpoint http://localhost:4318 --target env

Secret hygiene: header values whose NAMES contain bearer/token/key/
secret/password/authorization are written to the target file
verbatim but redacted to *** in 'mindspec otel status' output. To
keep secrets out of files entirely, prefer setting
OTEL_EXPORTER_OTLP_HEADERS at workload-launch time.

This command performs ZERO network I/O. It never probes the
endpoint, never validates connectivity, never spawns a collector.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint, _ := cmd.Flags().GetString("endpoint")
		protocol, _ := cmd.Flags().GetString("protocol")
		serviceName, _ := cmd.Flags().GetString("service-name")
		headersFlag, _ := cmd.Flags().GetString("headers")
		target, _ := cmd.Flags().GetString("target")
		codexFlag, _ := cmd.Flags().GetBool("codex")
		codexConfigPath, _ := cmd.Flags().GetString("codex-config")

		// --codex is a shorthand for --target=codex. If both are
		// supplied and disagree, --codex wins (it is the more specific
		// flag).
		if codexFlag {
			target = "codex"
		}
		if target == "" {
			target = "claude"
		}

		headers, err := otel.ParseHeaders(headersFlag)
		if err != nil {
			cmd.SilenceUsage = false
			return usageErr(err)
		}

		cfg := otel.Config{
			Endpoint:    endpoint,
			Protocol:    protocol,
			ServiceName: serviceName,
			Headers:     headers,
		}
		if err := cfg.Validate(); err != nil {
			cmd.SilenceUsage = false
			return usageErr(err)
		}

		switch target {
		case "claude":
			return runOtelSetupClaude(cmd, cfg)
		case "codex":
			return runOtelSetupCodex(cmd, cfg, codexConfigPath)
		case "env":
			fmt.Fprint(cmd.OutOrStdout(), otel.RenderEnvExports(cfg))
			return nil
		default:
			cmd.SilenceUsage = false
			return usageErr(fmt.Errorf("unknown --target %q (expected claude, codex, or env)", target))
		}
	},
}

func runOtelSetupClaude(cmd *cobra.Command, c otel.Config) error {
	root, err := findRoot()
	if err != nil {
		// Fall back to cwd: setup should work outside a mindspec
		// workspace (the caller may be configuring a fresh repo).
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return cwdErr
		}
		root = cwd
	}
	res, err := otel.WriteClaudeSettingsLocal(root, c)
	if err != nil {
		return err
	}
	if res.NoOp {
		fmt.Fprintf(cmd.OutOrStdout(), "OTEL config unchanged in %s\n", res.Path)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote OTEL config to %s\n", res.Path)
	}
	return nil
}

func runOtelSetupCodex(cmd *cobra.Command, c otel.Config, codexConfigPath string) error {
	res, err := otel.WriteCodexConfig(codexConfigPath, c)
	if err != nil {
		return err
	}
	if res.NoOp {
		fmt.Fprintf(cmd.OutOrStdout(), "OTEL config unchanged in %s\n", res.Path)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote OTEL config to %s\n", res.Path)
	}
	return nil
}

// otelStatusCmd implements `mindspec otel status` per spec 084. Reads
// only; writes only stdout/stderr; performs ZERO network I/O.
//
// Exit code matrix (spec Hard Constraint #5):
//
//	0 — config present and parseable in at least one target file
//	1 — no OTEL config found in any target file
//	2 — at least one target file exists but fails to parse
var otelStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Print the currently configured OTEL endpoint (read-only)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findRoot()
		if err != nil {
			cwd, cwdErr := os.Getwd()
			if cwdErr != nil {
				return cwdErr
			}
			root = cwd
		}
		codexConfigPath, _ := cmd.Flags().GetString("codex-config")

		var status otel.CurrentStatus
		if codexConfigPath != "" {
			status, _ = otel.ReadCurrentWithCodexPath(root, codexConfigPath)
		} else {
			status, _ = otel.ReadCurrent(root)
		}

		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "mindspec otel status")
		fmt.Fprintln(out, "====================")
		printTargetStatus(out, "claude", status.ClaudePath, status.ClaudePresent, status.ClaudeParseErr, status.Claude)
		printTargetStatus(out, "codex", status.CodexPath, status.CodexPresent, status.CodexParseErr, status.Codex)

		// Exit-code selection per the matrix.
		if status.ClaudeParseErr != "" || status.CodexParseErr != "" {
			return exitErr(2, fmt.Errorf("at least one target file failed to parse"))
		}
		if !status.ClaudePresent && !status.CodexPresent {
			return exitErr(1, fmt.Errorf("no OTEL config found in any target file"))
		}
		return nil
	},
}

// printTargetStatus emits a stable, human-readable status block for
// one target. The block shape is stable so it can be golden-tested in
// later beads.
func printTargetStatus(out interface{ Write([]byte) (int, error) }, label, path string, present bool, parseErr string, c otel.Config) {
	fmt.Fprintf(out, "\nTarget: %s\n  Path: %s\n", label, path)
	if parseErr != "" {
		fmt.Fprintf(out, "  Status: PARSE ERROR (%s)\n", parseErr)
		return
	}
	if !present {
		fmt.Fprintf(out, "  Status: not configured\n")
		return
	}
	r := c.Redacted()
	fmt.Fprintf(out, "  Status: configured\n")
	fmt.Fprintf(out, "  Endpoint: %s\n", r.Endpoint)
	if r.Protocol != "" {
		fmt.Fprintf(out, "  Protocol: %s\n", r.Protocol)
	}
	if r.ServiceName != "" {
		fmt.Fprintf(out, "  Service: %s\n", r.ServiceName)
	}
	if len(r.Headers) > 0 {
		keys := make([]string, 0, len(r.Headers))
		for k := range r.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(out, "  Headers:\n")
		for _, k := range keys {
			fmt.Fprintf(out, "    %s=%s\n", k, r.Headers[k])
		}
	}
}

// usageErr wraps an error so cobra exits with status 2 (invocation
// error) per spec Hard Constraint #5.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func usageErr(err error) error      { return &usageError{err: err} }

// exitErr wraps an error with a specific exit code. Cobra's default
// behavior is exit 1 on any RunE error; we need exits 0/1/2 per the
// spec matrix. main.go reads this type to set os.Exit appropriately.
type codedError struct {
	code int
	err  error
}

func (e *codedError) Error() string { return e.err.Error() }
func exitErr(code int, err error) error {
	return &codedError{code: code, err: err}
}

// ExitCode returns the exit code for an error, used by main.go. Returns
// 0 if err is nil, the wrapped code for *codedError, 2 for
// *usageError, the workload's exit code for *workloadExitError
// (spec 084 Bead 2 — `mindspec record start` propagates the workload
// exit code verbatim), and 1 for any other error (cobra default).
func otelExitCode(err error) int {
	if err == nil {
		return 0
	}
	switch e := err.(type) {
	case *codedError:
		return e.code
	case *usageError:
		return 2
	case *workloadExitError:
		return e.code
	}
	return 1
}

func init() {
	otelSetupCmd.Flags().String("endpoint", "", "OTLP endpoint URL (required, e.g. http://localhost:4318)")
	otelSetupCmd.Flags().String("protocol", "", "OTLP protocol: http/json (default), http/protobuf, or grpc")
	otelSetupCmd.Flags().String("service-name", "", "OTEL service.name attribute (default: mindspec)")
	otelSetupCmd.Flags().String("headers", "", "Comma-separated headers: k1=v1,k2=v2")
	otelSetupCmd.Flags().String("target", "", "Target: claude (default), codex, or env")
	otelSetupCmd.Flags().Bool("codex", false, "Shorthand for --target=codex")
	otelSetupCmd.Flags().String("codex-config", "", "Path to Codex config.toml (default: ~/.codex/config.toml)")

	otelStatusCmd.Flags().String("codex-config", "", "Path to Codex config.toml to inspect (default: ~/.codex/config.toml)")

	otelCmd.AddCommand(otelSetupCmd)
	otelCmd.AddCommand(otelStatusCmd)
}
