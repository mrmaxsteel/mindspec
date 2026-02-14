package viz

import (
	"context"
	"fmt"
	"os"
)

// RunLive creates the full live visualization pipeline and blocks until ctx is cancelled.
func RunLive(ctx context.Context, otlpPort, uiPort int) error {
	graph := NewGraph(DefaultGraphConfig())
	hub := NewHub()
	go hub.Run(ctx)

	server := NewServer(uiPort, hub, graph)
	go func() {
		if err := server.Run(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "UI server error: %v\n", err)
		}
	}()

	receiver := NewLiveReceiver(otlpPort, graph, hub)
	return receiver.Run(ctx)
}

// RunReplay creates the full replay visualization pipeline and blocks until ctx is cancelled.
func RunReplay(ctx context.Context, path string, speed float64, uiPort int) error {
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
	if err := replay.Run(ctx); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Replay done. Server still running at http://localhost:%d (Ctrl-C to stop)\n", uiPort)
	<-ctx.Done()
	return nil
}
