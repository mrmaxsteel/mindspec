BINARY  := mindspec
BINDIR  := ./bin
PKG     := ./cmd/mindspec

.PHONY: build install test bench-llm clean verify-agentmind-tag checkout-agentmind

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

clean:
	rm -rf $(BINDIR)
