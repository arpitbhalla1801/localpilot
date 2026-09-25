// Package testutil holds test helpers shared across internal packages
// that each need a real, killable listening process to test against
// (rather than mocking process/network inspection). It's a plain package,
// not a _test.go file, since Go test binaries can't import another
// package's unexported test-only symbols.
package testutil

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

// ListenHelperMain is the body of the re-exec'd helper process. A calling
// package wires it into its own test binary with:
//
//	func TestListenHelperProcess(t *testing.T) { testutil.ListenHelperMain() }
//
// StartListenerHelper re-execs the current test binary with
// -test.run=TestListenHelperProcess, so each package needs exactly that
// function name defined once.
func ListenHelperMain() {
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
	time.Sleep(time.Hour)
}

// FreePort finds a currently-unused TCP port by briefly binding to port 0.
func FreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// ListenerHelper is a subprocess listening on a real port, for tests that
// need to kill/watch/free a real process without racing the test binary's
// own listeners or risking the test binary killing itself.
type ListenerHelper struct {
	Cmd *exec.Cmd
	out *bufio.Reader
}

// StartListenerHelper spawns the current test binary re-exec'd into
// TestListenHelperProcess (see ListenHelperMain), listening on port. The
// helper process is killed automatically via t.Cleanup; callers don't
// need their own defer/cleanup for it.
func StartListenerHelper(t *testing.T, port int) *ListenerHelper {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestListenHelperProcess")
	cmd.Env = append(os.Environ(), fmt.Sprintf("LOCALPILOT_TEST_LISTEN_PORT=%d", port))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	h := &ListenerHelper{Cmd: cmd, out: bufio.NewReader(stdout)}
	t.Cleanup(h.Stop)
	return h
}

// waitOr blocks until ch delivers, or fails the test with msg on timeout.
func waitOr(t *testing.T, ch <-chan error, timeout time.Duration, msg string) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		t.Fatal(msg)
		return nil
	}
}

// WaitListening blocks until the helper process reports it's listening,
// or fails the test if it doesn't within the timeout.
func (h *ListenerHelper) WaitListening(t *testing.T) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := h.out.ReadString('\n')
		done <- err
	}()
	if err := waitOr(t, done, 5*time.Second, "timed out waiting for helper process to listen"); err != nil {
		t.Fatalf("helper process did not report listening: %v", err)
	}
}

// WaitExit blocks until the helper process exits, or fails the test if it
// doesn't within timeout (e.g. because a kill/free didn't work).
func (h *ListenerHelper) WaitExit(t *testing.T, timeout time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- h.Cmd.Wait() }()
	waitOr(t, done, timeout, "helper process did not exit in time")
}

// Stop kills the helper process if it's still running.
func (h *ListenerHelper) Stop() {
	if h.Cmd.Process != nil {
		_ = h.Cmd.Process.Kill()
		_ = h.Cmd.Wait()
	}
}
