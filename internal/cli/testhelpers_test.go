package cli

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// TestListenHelperProcess is not a real test. It is re-exec'd as a
// subprocess (the standard os/exec "helper process" pattern) so other
// tests can exercise commands against a real, killable process listening
// on a real port, instead of the test binary itself.
func TestListenHelperProcess(t *testing.T) {
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

func itoa(n int) string { return strconv.Itoa(n) }

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

type listenerHelper struct {
	t   *testing.T
	cmd *exec.Cmd
	out *bufio.Reader
}

// startListenerHelper spawns a subprocess listening on port so tests can
// kill/watch/free a real process without racing the test binary's own
// listeners or risking the test binary killing itself.
func startListenerHelper(t *testing.T, port int) *listenerHelper {
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
	return &listenerHelper{t: t, cmd: cmd, out: bufio.NewReader(stdout)}
}

func (h *listenerHelper) waitListening(t *testing.T) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := h.out.ReadString('\n')
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
}

// waitExit blocks until the helper process exits, or fails the test if it
// doesn't within the timeout (e.g. because a kill/free didn't work).
func (h *listenerHelper) waitExit(t *testing.T, timeout time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- h.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("helper process did not exit in time")
	}
}

func (h *listenerHelper) stop() {
	if h.cmd.Process != nil {
		_ = h.cmd.Process.Kill()
		_ = h.cmd.Wait()
	}
}
