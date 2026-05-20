// Package livecapture hosts spec 083 Test D (live-capture) gates. The
// tests in this package are guarded by the `livecapture` build tag so
// they do NOT run during `go test -short ./...`. CI invokes them via
// the dedicated `make test-live-capture` target, which sets
// $AGENTMIND_BIN to a real or fake agentmind binary and runs:
//
//	go test -tags=livecapture -run TestLiveCapture ./internal/specgate/livecapture/...
//
// Per spec 083 plan Bead 4 step 3:
//
//	"Add a CI job that runs scripts/checkout-agentmind.sh, builds the
//	 agentmind binary from the sibling checkout, runs `agentmind serve
//	 --otlp-port <p> --ui-port 0 --output /tmp/em.ndjson &`, POSTs a
//	 synthetic OTLP log payload via curl, runs the agentmind/wire
//	 normalization tool against /tmp/em.ndjson … and diffs the
//	 normalized output against a reference fixture."
//
// During Phase 4 (the current phase), the agentmind sibling repo does
// not yet publish a real `cmd/agentmind` binary that parses OTLP. The
// in-repo `cmd/agentmind-fake` is the stand-in: it binds the OTLP
// port (so AutoStart's WaitForPort succeeds), emits N synthetic
// `wire.CollectedEvent` records to stdout and to --output, and
// honors --linger so the parent stays in control.
//
// The live-capture gate therefore has two assertion levels:
//
//   - Always: the binary at $AGENTMIND_BIN can be started, binds the
//     OTLP port, and writes NDJSON to --output containing at least
//     one valid `wire.CollectedEvent`.
//   - When $AGENTMIND_REAL_BINARY=1: ALSO POST a synthetic OTLP log
//     payload to the OTLP port and assert the resulting NDJSON
//     contains an event with `name = "claude_code.api_request"` (the
//     spec-canonical real-binary assertion). The fake never sets
//     this env var because it cannot satisfy the OTLP-parsing half.
//
// CI gating: per spec Test G, this job is skipped (not failed) when
// the agentmind v0.0.1 tag is not reachable upstream. The Makefile
// target prints a diagnostic and exits 0 in that case so CI can
// continue.
package livecapture
