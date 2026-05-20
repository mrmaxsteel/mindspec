BINARY  := mindspec
BINDIR  := ./bin
PKG     := ./cmd/mindspec

.PHONY: build install test bench-llm clean

build:
	go build -o $(BINDIR)/$(BINARY) $(PKG)

install:
	go install $(PKG)

test:
	go test -short ./...

bench-llm:
	go test ./internal/harness/ -v -run TestLLM -timeout 600s

clean:
	rm -rf $(BINDIR)
