package cli

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/localpilot/localpilot/internal/agent"
)

// TestFreeHelperProcess is not a real test. It is re-exec'd as a subprocess
// by TestFreeCmd_KillsListener (the standard os/exec "helper process"
// pattern) to listen on a port so free can be exercised against a real,
// killable process instead of the test binary itself.
func TestFreeHelperProcess(t *testing.T) {
	port := os.Getenv("LOCALPILOT_TEST_LISTEN_PORT")
	if port == "" {
		return
	}
	l, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer l.Close()
	fmt.Println("listening")
	select {}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestFreeCmd_KillsListener(t *testing.T) {
	if _, err := agent.New(); err != nil {
		t.Skipf("agent not available on this platform: %v", err)
	}

	port := freePort(t)

	helper := exec.Command(os.Args[0], "-test.run=TestFreeHelperProcess")
	helper.Env = append(os.Environ(), fmt.Sprintf("LOCALPILOT_TEST_LISTEN_PORT=%d", port))
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	defer func() {
		_ = helper.Process.Kill()
		_ = helper.Wait()
	}()

	// Wait for the helper to confirm it's actually listening before we
	// race a `free` against it.
	buf := make([]byte, len("listening\n"))
	done := make(chan error, 1)
	go func() {
		_, err := stdout.Read(buf)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("helper process did not report listening: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for helper process to listen")
	}

	a, err := agent.New()
	if err != nil {
		t.Fatalf("agent.New: %v", err)
	}
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

	waitErr := make(chan error, 1)
	go func() { waitErr <- helper.Wait() }()
	select {
	case <-waitErr:
		// Process exited, as expected.
	case <-time.After(5 * time.Second):
		t.Fatal("helper process was not terminated by free")
	}

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
