BINARY  := mindspec
BINDIR  := ./bin
PKG     := ./cmd/mindspec

.PHONY: build install test bench-llm clean verify-agentmind-tag checkout-agentmind verify-sibling test-live-capture

build:
	go build -o $(BINDIR)/$(BINARY) $(PKG)

install:
	go install $(PKG)

test:
	go test -short ./...

bench-llm:
	go test ./internal/harness/ -v -run TestLLM -timeout 600s

# Spec 083 / Test G — Phase 0 prerequisite gate. Verifies that
# github.com/mrmaxsteel/agentmind has published the v0.0.1 tag. Exits 0 when
# the tag is found (prints SHA to stdout); exits 2 when the repo is reachable
# but the tag is absent; exits 3 when the repo itself is unreachable.
verify-agentmind-tag:
	./scripts/verify-agentmind-tag.sh

# Spec 083 Bead 2 — sibling-checkout helper. Ensures the agentmind sibling
# repo exists at ../agentmind so the go.mod `replace` directive resolves,
# and writes a gitignored go.work file at the module root pinning the
# sibling via an absolute path (so worktree depth does not affect
# resolution). Exits 0 (sibling present), 2 (tag absent upstream),
# 3 (upstream unreachable).
checkout-agentmind:
	./scripts/checkout-agentmind.sh

# Spec 083 Bead 3a — panel bead-3a-v1 REV-6 sibling cross-check.
# Confirms the agentmind sibling resolves, compiles, and passes its
# own `go test -short ./...` gate. Without this, reviewers seeing only
# mindspec's diff cannot independently verify ErrBinaryNotFound /
# EmitWarnOnce / findBinary in the out-of-tree sibling.
verify-sibling:
	./scripts/verify-sibling.sh

# Spec 083 Bead 4 — Test D (live capture) gate. Runs the end-to-end
# live-capture flow: builds the in-repo cmd/agentmind-fake (or honors
# a preset $AGENTMIND_BIN pointing at a real binary), starts it via
# client.AutoStart, POSTs a synthetic OTLP log payload, then asserts
# the resulting NDJSON --output file contains at least one valid
# wire.CollectedEvent. The test is guarded by the `livecapture`
# build tag so it does NOT run during `go test -short ./...`; this
# target is the only CI entry point.
#
# Set AGENTMIND_REAL_BINARY=1 to enforce the spec-canonical assertion
# that NDJSON contains an event with name="claude_code.api_request".
# The fake cannot satisfy that half (it does not parse OTLP), so the
# Makefile target leaves it unset by default.
#
# Test G gating: when the Phase 0 prerequisite gate exits non-zero
# (agentmind v0.0.1 tag absent upstream), this target prints a
# diagnostic and exits 0 so CI can continue. CI sets
# MINDSPEC_REQUIRE_LIVE_CAPTURE=1 to turn that into a hard failure
# once the upstream tag exists.
test-live-capture:
	@set -e; \
	if [ -z "$$AGENTMIND_BIN" ]; then \
		echo "==> building cmd/agentmind-fake as AGENTMIND_BIN for live-capture gate"; \
		mkdir -p $(BINDIR); \
		go build -o $(BINDIR)/agentmind-fake ./cmd/agentmind-fake; \
		export AGENTMIND_BIN="$$PWD/$(BINDIR)/agentmind-fake"; \
		echo "==> AGENTMIND_BIN=$$AGENTMIND_BIN"; \
		go test -tags=livecapture -count=1 -timeout 60s -run TestLiveCapture ./internal/specgate/livecapture/...; \
	else \
		echo "==> using preset AGENTMIND_BIN=$$AGENTMIND_BIN"; \
		go test -tags=livecapture -count=1 -timeout 60s -run TestLiveCapture ./internal/specgate/livecapture/...; \
	fi

clean:
	rm -rf $(BINDIR)
