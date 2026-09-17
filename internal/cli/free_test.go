package cli

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/localpilot/localpilot/internal/agent"
)

func TestFreeCmd_KillsListener(t *testing.T) {
	a, err := agent.New()
	if err != nil {
		t.Skipf("agent not available on this platform: %v", err)
	}

	port := freePort(t)
	helper := startListenerHelper(t, port)
	defer helper.stop()
	helper.waitListening(t)

	binding, err := a.FindPort(context.Background(), port)
	if err != nil || !binding.InUse {
		t.Fatalf("expected helper process to be listening on port %d (binding=%+v, err=%v)", port, binding, err)
	}

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"free", strconv.Itoa(port), "--force"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("free --force failed: %v", err)
	}

	helper.waitExit(t, 5*time.Second)

	binding, err = a.FindPort(context.Background(), port)
	if err != nil {
		t.Fatalf("FindPort after free: %v", err)
	}
	if binding.InUse {
		t.Errorf("port %d still reported in use after free", port)
	}
}

func TestFreeCmd_NotInUse(t *testing.T) {
	if _, err := agent.New(); err != nil {
		t.Skipf("agent not available on this platform: %v", err)
	}

	port := freePort(t)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"free", strconv.Itoa(port), "--force"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("free on an unused port should not error, got: %v", err)
	}
}

func TestFreeCmd_InvalidPort(t *testing.T) {
	rootCmd.SetArgs([]string{"free", "not-a-port", "--force"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}
