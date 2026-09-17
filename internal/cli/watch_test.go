package cli

import (
	"context"
	"testing"
	"time"

	"github.com/localpilot/localpilot/internal/agent"
)

func TestResolveWatchTarget_Port(t *testing.T) {
	a, err := agent.New()
	if err != nil {
		t.Skipf("agent not available on this platform: %v", err)
	}

	port := freePort(t)

	helper := startListenerHelper(t, port)
	defer helper.stop()
	helper.waitListening(t)

	gotPort, gotPID, isPort, err := resolveWatchTarget(context.Background(), a, itoa(port))
	if err != nil {
		t.Fatalf("resolveWatchTarget: %v", err)
	}
	if !isPort {
		t.Errorf("expected isPort=true for a listening port")
	}
	if gotPort != port {
		t.Errorf("gotPort = %d, want %d", gotPort, port)
	}
	if gotPID != 0 {
		t.Errorf("gotPID = %d, want 0 when resolved as a port", gotPID)
	}
}

func TestResolveWatchTarget_PID(t *testing.T) {
	a, err := agent.New()
	if err != nil {
		t.Skipf("agent not available on this platform: %v", err)
	}

	// A number that isn't a valid port (>65535) but is a valid PID falls
	// back to PID interpretation, same disambiguation rule as `kill`.
	_, gotPID, isPort, err := resolveWatchTarget(context.Background(), a, "999999999")
	if err != nil {
		t.Fatalf("resolveWatchTarget: %v", err)
	}
	if isPort {
		t.Errorf("expected isPort=false when target is out of port range")
	}
	if gotPID != 999999999 {
		t.Errorf("gotPID = %d, want 999999999", gotPID)
	}
}

func TestResolveWatchTarget_InvalidTarget(t *testing.T) {
	a, err := agent.New()
	if err != nil {
		t.Skipf("agent not available on this platform: %v", err)
	}

	if _, _, _, err := resolveWatchTarget(context.Background(), a, "not-a-number"); err == nil {
		t.Fatal("expected error for a non-numeric target")
	}
}

func TestRunWatch_TicksUntilCancelled(t *testing.T) {
	a, err := agent.New()
	if err != nil {
		t.Skipf("agent not available on this platform: %v", err)
	}

	port := freePort(t)
	helper := startListenerHelper(t, port)
	defer helper.stop()
	helper.waitListening(t)

	watchInterval = 20 * time.Millisecond
	defer func() { watchInterval = time.Second }()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := runWatch(ctx, a, itoa(port)); err != nil {
		t.Fatalf("runWatch: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("runWatch returned too early (%v); expected it to keep ticking until context deadline", elapsed)
	}
}
