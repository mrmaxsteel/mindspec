module github.com/mrmaxsteel/mindspec

go 1.23.0

require (
	github.com/mrmaxsteel/agentmind v0.0.0-00010101000000-000000000000
	github.com/spf13/cobra v1.8.1
	golang.org/x/sys v0.31.0
	gopkg.in/yaml.v3 v3.0.1
	nhooyr.io/websocket v1.8.17
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

// Spec 083 Bead 2: local `replace` directive used through Phases 2-5.
// Removed in Phase 6 (Bead 6) once agentmind v1.0.0 ships upstream and
// the require line is bumped to the released tag.
//
// During Phase 0/1 (before agentmind v0.1.0 is published upstream), the
// require version is a zero pseudo-version because the replace directive
// fully overrides it. CI (Bead 4 onward) materializes the sibling
// checkout via scripts/checkout-agentmind.sh before running `go build`.
replace github.com/mrmaxsteel/agentmind => ../agentmind
