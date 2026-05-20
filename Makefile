BINARY  := mindspec
BINDIR  := ./bin
PKG     := ./cmd/mindspec

.PHONY: build install test bench-llm clean verify-agentmind-tag

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

clean:
	rm -rf $(BINDIR)
