package viz

import (
	"context"
	"testing"
	"time"
)

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Simulate a client with a send channel
	c := &Client{
		hub:  hub,
		send: make(chan []byte, 64),
	}
	hub.register <- c

	// Wait for registration
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}

	// Broadcast a message
	hub.Broadcast(WSMessage{Type: MsgStats, Data: "test"})

	select {
	case msg := <-c.send:
		if len(msg) == 0 {
			t.Error("expected non-empty message")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
	}
}

func TestHubDropped(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Create client with tiny buffer
	c := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
	}
	hub.register <- c
	time.Sleep(10 * time.Millisecond)

	// Fill the buffer
	hub.Broadcast(WSMessage{Type: MsgStats, Data: "fill"})
	time.Sleep(10 * time.Millisecond)

	// This should drop
	hub.Broadcast(WSMessage{Type: MsgStats, Data: "drop"})
	time.Sleep(10 * time.Millisecond)

	if hub.Dropped() == 0 {
		t.Error("expected dropped count > 0")
	}
}

func TestHubUnregister(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	c := &Client{
		hub:  hub,
		send: make(chan []byte, 64),
	}
	hub.register <- c
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.unregister <- c
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients after unregister, got %d", hub.ClientCount())
	}
}
