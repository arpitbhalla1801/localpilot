package platform

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/arpitbhalla1801/localpilot/internal/testutil"
)

func TestIdleBetween(t *testing.T) {
	read := func(cpu float64, io uint64) activitySample {
		return activitySample{cpu: cpu, io: io, cpuRead: true, ioRead: true}
	}
	tests := []struct {
		name string
		a, b activitySample
		idle bool
	}{
		{"no progress", read(1, 10), read(1, 10), true},
		{"cpu progress", read(1, 10), read(1.5, 10), false},
		{"io progress", read(1, 10), read(1, 99), false},
		{"cpu only, idle", activitySample{cpu: 1, cpuRead: true}, activitySample{cpu: 1, cpuRead: true}, true},
		{"nothing readable", activitySample{}, activitySample{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := idleBetween(tt.a, tt.b); got != tt.idle {
				t.Errorf("idleBetween = %v, want %v", got, tt.idle)
			}
		})
	}
}

// A process blocked in accept() with no connections is the canonical idle
// listener; once a client connects, it must no longer be reported idle.
func TestIdlePIDs_Listener(t *testing.T) {
	port := testutil.FreePort(t)
	h := testutil.StartListenerHelper(t, port)
	h.WaitListening(t)
	pid := int32(h.Cmd.Process.Pid)

	if !IdlePIDs(context.Background(), []int32{pid})[pid] {
		t.Fatal("idle listener not reported idle")
	}

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if IdlePIDs(context.Background(), []int32{pid})[pid] {
		t.Error("listener with an established connection reported idle")
	}
}
