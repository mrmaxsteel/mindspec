package viz

import (
	"context"
	"fmt"
	"os"
)

// LiveOpts holds options for RunLive.
type LiveOpts struct {
	OTLPPort   int
	UIPort     int
	OutputPath string
	BindAddr   string // default "127.0.0.1"
}

// RunLive creates the full live visualization pipeline and blocks until ctx is cancelled.
// If outputPath is non-empty, events are also written to an NDJSON file on disk.
func RunLive(ctx context.Context, otlpPort, uiPort int, outputPath string) error {
	return RunLiveOpts(ctx, LiveOpts{
		OTLPPort:   otlpPort,
		UIPort:     uiPort,
		OutputPath: outputPath,
	})
}

// RunLiveOpts creates the full live visualization pipeline with extended options.
func RunLiveOpts(ctx context.Context, opts LiveOpts) error {
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()
	go hub.Run(ctx)

	receiver := NewLiveReceiver(opts.OTLPPort, graph, hub)
	if opts.BindAddr != "" {
		receiver.SetBindAddr(opts.BindAddr)
	}
	if opts.OutputPath != "" {
		receiver.SetOutput(opts.OutputPath)
	}

	server := NewServer(opts.UIPort, hub, graph)
	if opts.BindAddr != "" {
		server.SetBindAddr(opts.BindAddr)
	}
	server.SetLiveReceiver(receiver)
	go func() {
		if err := server.Run(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "UI server error: %v\n", err)
		}
	}()

	return receiver.Run(ctx)
}

// RunReplay creates the full replay visualization pipeline and blocks until ctx is cancelled.
// If phase is non-empty, only events within that lifecycle phase are replayed.
func RunReplay(ctx context.Context, path string, speed float64, uiPort int, phase string) error {
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()
	go hub.Run(ctx)

	server := NewServer(uiPort, hub, graph)
	go func() {
		if err := server.Run(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "UI server error: %v\n", err)
		}
	}()

	replay := NewReplay(path, speed, graph, hub)
	replay.phase = phase
	if err := replay.Run(ctx); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Replay done. Server still running at http://localhost:%d (Ctrl-C to stop)\n", uiPort)
	<-ctx.Done()
	return nil
}
