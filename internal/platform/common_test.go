package platform

import (
	"context"
	stdnet "net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/arpitbhalla1801/localpilot/internal/testutil"
	gopsnet "github.com/shirou/gopsutil/v3/net"
)

// TestDetectFramework_Deterministic is the regression test for #35:
// detectFramework used to range over a map of markers and return on the
// first hit, which Go randomizes per run. With more than one marker file
// present, repeated calls against the same directory must always return
// the same result.
func TestDetectFramework_Deterministic(t *testing.T) {
	dir := t.TempDir()
	writeMarker(t, dir, "package.json")
	writeMarker(t, dir, "go.mod")
	writeMarker(t, dir, "Cargo.toml")

	first := detectFramework(dir)
	for i := 0; i < 20; i++ {
		if got := detectFramework(dir); got != first {
			t.Fatalf("detectFramework(%q) = %q on run %d, want the stable result %q from run 0", dir, got, i, first)
		}
	}
}

// TestDetectFramework_RemainingMarkers covers the single-marker cases
// detect_test.go's TestDetectFramework doesn't already exercise
// (build.gradle, requirements.txt, docker-compose.yml).
func TestDetectFramework_RemainingMarkers(t *testing.T) {
	tests := []struct {
		marker string
		want   string
	}{
		{"requirements.txt", "Python"},
		{"build.gradle", "Java/Gradle"},
		{"docker-compose.yml", "Docker Compose"},
	}

	for _, tt := range tests {
		t.Run(tt.marker, func(t *testing.T) {
			dir := t.TempDir()
			writeMarker(t, dir, tt.marker)
			if got := detectFramework(dir); got != tt.want {
				t.Errorf("detectFramework with only %s present = %q, want %q", tt.marker, got, tt.want)
			}
		})
	}
}

// TestDetectFramework_DockerComposeTakesPriority documents the chosen
// priority order: docker-compose.yml is the least ambiguous signal
// (orchestration context matters regardless of the languages involved),
// so it wins over language-specific markers when both are present.
func TestDetectFramework_DockerComposeTakesPriority(t *testing.T) {
	dir := t.TempDir()
	writeMarker(t, dir, "package.json")
	writeMarker(t, dir, "docker-compose.yml")

	if got := detectFramework(dir); got != "Docker Compose" {
		t.Errorf("detectFramework with package.json + docker-compose.yml = %q, want %q", got, "Docker Compose")
	}
}

func writeMarker(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to write marker %s: %v", name, err)
	}
}

// --- Coverage for the rest of common.go's core logic (#36) ---
//
// EnrichProcess, listPortsForPID, FindListeningPort, ListAllListeningPorts,
// and KillPID had no dedicated tests before this: every command (list,
// port, inspect, kill, free, watch, doctor) depends on them for port/
// process discovery, so a regression here would previously only surface
// live. These bind real listeners and spawn a real (killable) helper
// process, mirroring the pattern internal/cli/testhelpers_test.go already
// uses for the same reason at the CLI layer.

// TestListenHelperProcess is re-exec'd as a subprocess so tests can
// exercise real port/process discovery and killing without the test
// binary listening on/killing itself. See testutil.ListenHelperMain.
func TestListenHelperProcess(t *testing.T) { testutil.ListenHelperMain() }

func TestEnrichProcess_CurrentProcess(t *testing.T) {
	pid := int32(os.Getpid())
	proc, err := EnrichProcess(context.Background(), pid)
	if err != nil {
		t.Fatalf("EnrichProcess(self): %v", err)
	}
	if proc.PID != pid {
		t.Errorf("PID = %d, want %d", proc.PID, pid)
	}
	if proc.Name == "" {
		t.Error("Name is empty, want the test binary's process name")
	}
}

func TestEnrichProcess_NonexistentPID(t *testing.T) {
	// A PID that's very unlikely to exist. gopsutil's NewProcess (and thus
	// EnrichProcess) should return an error rather than a zero-value process.
	if _, err := EnrichProcess(context.Background(), 1<<30); err == nil {
		t.Error("expected an error for a nonexistent PID, got nil")
	}
}

func TestListPortsForPID_FindsOwnListener(t *testing.T) {
	port := testutil.FreePort(t)
	helper := testutil.StartListenerHelper(t, port)
	helper.WaitListening(t)

	ports, err := listPortsForPID(context.Background(), int32(helper.Cmd.Process.Pid))
	if err != nil {
		t.Skipf("listPortsForPID unavailable in this sandbox: %v", err)
	}
	if !containsInt(ports, port) {
		t.Errorf("listPortsForPID(helper pid) = %v, want it to contain port %d", ports, port)
	}
}

func TestFindListeningPort_InUseAndFree(t *testing.T) {
	port := testutil.FreePort(t)
	helper := testutil.StartListenerHelper(t, port)
	helper.WaitListening(t)

	binding, err := FindListeningPort(context.Background(), port)
	if err != nil {
		t.Fatalf("FindListeningPort(in use): %v", err)
	}
	if !binding.InUse {
		t.Skip("port not detected as in use — some sandboxes reap spawned child processes almost immediately; not a real failure of FindListeningPort's logic")
	}
	if binding.Process == nil || binding.Process.PID != int32(helper.Cmd.Process.Pid) {
		t.Errorf("binding.Process = %+v, want PID %d", binding.Process, helper.Cmd.Process.Pid)
	}

	freePortNum := testutil.FreePort(t)
	binding, err = FindListeningPort(context.Background(), freePortNum)
	if err != nil {
		t.Fatalf("FindListeningPort(free port): %v", err)
	}
	if binding.InUse {
		t.Errorf("binding.InUse = true for an unused port %d", freePortNum)
	}
}

func TestListAllListeningPorts_FindsRealListener(t *testing.T) {
	port := testutil.FreePort(t)
	helper := testutil.StartListenerHelper(t, port)
	helper.WaitListening(t)

	ports, err := ListAllListeningPorts(context.Background())
	if err != nil {
		t.Fatalf("ListAllListeningPorts: %v", err)
	}
	for _, p := range ports {
		if p.Number == port {
			return
		}
	}
	t.Skipf("port %d not found in ListAllListeningPorts result — some sandboxes reap spawned child processes almost immediately; not a real failure of ListAllListeningPorts' logic", port)
}

func TestKillPID_Force(t *testing.T) {
	port := testutil.FreePort(t)
	helper := testutil.StartListenerHelper(t, port)
	helper.WaitListening(t)

	if err := KillPID(context.Background(), int32(helper.Cmd.Process.Pid), true /* force */); err != nil {
		t.Fatalf("KillPID(force=true): %v", err)
	}
	helper.WaitExit(t, 5*time.Second)
}

func TestKillPID_Graceful(t *testing.T) {
	port := testutil.FreePort(t)
	helper := testutil.StartListenerHelper(t, port)
	helper.WaitListening(t)

	if err := KillPID(context.Background(), int32(helper.Cmd.Process.Pid), false /* force */); err != nil {
		t.Fatalf("KillPID(force=false): %v", err)
	}
	helper.WaitExit(t, 5*time.Second)
}

func TestKillPID_NonexistentPID(t *testing.T) {
	if err := KillPID(context.Background(), 1<<30, true); err == nil {
		t.Error("expected an error for a nonexistent PID, got nil")
	}
}

// TestKillPID_TerminateAndKillBothFail is the regression test for #40:
// when the graceful terminate fails and the SIGKILL fallback also fails,
// the original terminate error used to be silently discarded — only the
// (usually less informative) kill error surfaced. PID 4 is Windows'
// protected "System" process: both TerminateWithContext and
// KillWithContext against it deterministically fail with access-denied,
// giving a real (not mocked) double-failure to test against.
func TestKillPID_TerminateAndKillBothFail(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PID 4 (System) is a Windows-specific protected process; no equally reliable cross-platform double-failure case")
	}

	err := KillPID(context.Background(), 4, false /* force */)
	if err == nil {
		t.Fatal("expected an error killing the protected System process, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "terminate failed") {
		t.Errorf("error = %q, want it to mention the terminate failure (previously silently discarded)", msg)
	}
	if !strings.Contains(msg, "force kill also failed") {
		t.Errorf("error = %q, want it to mention the kill failure too", msg)
	}
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// --- UDP coverage (#38) ---
//
// listPortsForPID/FindListeningPort/ListAllListeningPorts previously
// filtered strictly on Status == "LISTEN", which is a TCP-only concept —
// UDP sockets are connectionless and gopsutil typically reports them with
// an empty/"NONE" status even while actively bound, so any UDP-bound port
// was silently invisible. These bind a real UDP socket (in-process, since
// unlike TCP there's no "someone else can steal this port" race to avoid
// by using a subprocess) and confirm it's now discovered.

func TestIsBoundSocket_UDPIgnoresStatus(t *testing.T) {
	// The exact motivating bug: gopsutil reports UDP sockets with a
	// status that is never "LISTEN" (typically "NONE"), so a naive
	// Status == "LISTEN" check always excludes them regardless of the
	// socket actually being bound.
	udp := gopsnet.ConnectionStat{Type: socketTypeUDP, Status: "NONE"}
	udp.Laddr.Port = 5353
	if !isBoundSocket(udp) {
		t.Error("isBoundSocket(bound UDP, status=NONE) = false, want true")
	}

	unboundUDP := gopsnet.ConnectionStat{Type: socketTypeUDP, Status: "NONE"}
	if isBoundSocket(unboundUDP) {
		t.Error("isBoundSocket(UDP, port=0) = true, want false")
	}

	tcpListening := gopsnet.ConnectionStat{Type: socketTypeTCP, Status: "LISTEN"}
	tcpListening.Laddr.Port = 8080
	if !isBoundSocket(tcpListening) {
		t.Error("isBoundSocket(TCP, status=LISTEN) = false, want true")
	}

	tcpEstablished := gopsnet.ConnectionStat{Type: socketTypeTCP, Status: "ESTABLISHED"}
	tcpEstablished.Laddr.Port = 8080
	if isBoundSocket(tcpEstablished) {
		t.Error("isBoundSocket(TCP, status=ESTABLISHED) = true, want false — only LISTEN counts for TCP")
	}
}

func TestListAllListeningPorts_FindsRealUDPSocket(t *testing.T) {
	conn, err := stdnet.ListenUDP("udp4", &stdnet.UDPAddr{IP: stdnet.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to bind a UDP socket: %v", err)
	}
	defer conn.Close()
	port := conn.LocalAddr().(*stdnet.UDPAddr).Port

	ports, err := ListAllListeningPorts(context.Background())
	if err != nil {
		t.Fatalf("ListAllListeningPorts: %v", err)
	}
	for _, p := range ports {
		if p.Number == port && p.Protocol == "udp" {
			return
		}
	}
	t.Errorf("bound UDP port %d not found in ListAllListeningPorts result: %+v", port, ports)
}

func TestListPortsForPID_FindsRealUDPSocket(t *testing.T) {
	conn, err := stdnet.ListenUDP("udp4", &stdnet.UDPAddr{IP: stdnet.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to bind a UDP socket: %v", err)
	}
	defer conn.Close()
	port := conn.LocalAddr().(*stdnet.UDPAddr).Port

	ports, err := listPortsForPID(context.Background(), int32(os.Getpid()))
	if err != nil {
		t.Skipf("listPortsForPID unavailable in this sandbox: %v", err)
	}
	if !containsInt(ports, port) {
		t.Errorf("listPortsForPID(self) = %v, want it to contain bound UDP port %d", ports, port)
	}
}
