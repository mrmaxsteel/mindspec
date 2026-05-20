// Command agentmind-fake is a test-only stand-in for the real
// agentmind binary, used by spec 083 Bead 3b's consumer rewire tests.
//
// It mimics the contract that `agentmind/client.AutoStart` expects:
//
//   - Binds a TCP listener on `--otlp-port` so the parent's
//     WaitForPort check passes (the listener does not actually parse
//     OTLP — it only accepts connections so the port is "up").
//   - Emits a deterministic NDJSON stream of `wire.CollectedEvent`
//     records to stdout, one event per line, terminated by '\n'
//     (Hard Constraint #3). Each event has a unique `event` value so
//     consumer tests can assert receipt in order.
//   - Honors `--stdout-stream` (default behavior; the flag is
//     accepted for forward-compatibility with the real binary's
//     command-line surface — see client/autostart.go).
//   - Honors `--output <path>` by ALSO writing the same NDJSON to
//     that file when set (so the real-binary "writes both stdout and
//     `--output`" semantic is testable).
//   - Honors `--events <n>` (test-only flag; the real binary streams
//     forever) to emit a finite number of events and then exit
//     cleanly. Default is 3.
//
// Tests build this binary via `go build -o <tmpdir>/agentmind` and
// set `AGENTMIND_BIN` to that path so `client.AutoStart` resolves it
// in step 1 of the documented lookup order.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/mrmaxsteel/agentmind/wire"
)

func main() {
	// agentmind/client.AutoStart invokes the binary as
	//   <bin> serve --otlp-port=… --ui-port=… --stdout-stream [--output=…]
	// so we skip the leading "serve" subcommand token if present
	// before letting flag.Parse consume the flag-style args. This
	// mirrors the real agentmind cobra layout.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}

	fs := flag.NewFlagSet("agentmind-fake", flag.ExitOnError)
	otlpPort := fs.Int("otlp-port", 4318, "OTLP port to bind for the parent's WaitForPort check")
	uiPort := fs.Int("ui-port", 8420, "UI port (ignored by the fake)")
	outputPath := fs.String("output", "", "optional NDJSON output file path")
	stdoutStream := fs.Bool("stdout-stream", false, "emit NDJSON to stdout (Bead 3b read-side contract)")
	nEvents := fs.Int("events", 3, "number of synthetic events to emit before exit")
	// linger keeps the listener up after emission. Default 30s so the
	// fake mimics the real agentmind binary's long-running server
	// shape — tests can override via --linger=0 if they want a quick
	// exit.
	linger := fs.Duration("linger", 30*time.Second, "keep the listener up for this duration after emission")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "agentmind-fake: parse: %v\n", err)
		os.Exit(1)
	}
	_ = uiPort
	_ = flag.CommandLine // keep import non-trivial

	// Bind the OTLP port on the IPv4 loopback. WaitForPort dials
	// "localhost:<port>" which Go's net package resolves to
	// 127.0.0.1 first by default — binding 127.0.0.1 explicitly is
	// reliable across macOS and Linux runners.
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *otlpPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "agentmind-fake: bind :%d: %v\n", *otlpPort, err)
		os.Exit(1)
	}
	defer ln.Close()
	go func() {
		for {
			c, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			_ = c.Close()
		}
	}()

	// Optional file sink (mirrors the real binary's --output behavior).
	var fileSink io.Writer = io.Discard
	if *outputPath != "" {
		f, fileErr := os.Create(*outputPath)
		if fileErr != nil {
			fmt.Fprintf(os.Stderr, "agentmind-fake: open %s: %v\n", *outputPath, fileErr)
			os.Exit(1)
		}
		defer f.Close()
		fileSink = f
	}

	// Emit `n` synthetic wire.CollectedEvent records.
	out := io.MultiWriter(os.Stdout, fileSink)
	if !*stdoutStream {
		// If --stdout-stream is not set, write only to the file sink
		// (so the fake can also be invoked in a "legacy" mode that
		// matches the pre-Bead-3b path).
		out = fileSink
	}
	for i := 0; i < *nEvents; i++ {
		ev := wire.CollectedEvent{
			TS:    time.Unix(0, int64(i)*int64(time.Millisecond)).UTC().Format(time.RFC3339Nano),
			Event: fmt.Sprintf("fake.event.%d", i),
			Data: map[string]any{
				"index": i,
			},
			Resource: map[string]any{
				"bench.label": "fake",
			},
		}
		line, marshalErr := json.Marshal(ev)
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "agentmind-fake: marshal: %v\n", marshalErr)
			os.Exit(1)
		}
		if _, writeErr := out.Write(append(line, '\n')); writeErr != nil {
			// Parent closed the pipe — exit cleanly.
			return
		}
	}
	if *linger > 0 {
		time.Sleep(*linger)
	}
}
