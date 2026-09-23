package platform

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// wslLookupTimeout mirrors dockerLookupTimeout: bound how long we'll wait
// on wsl.exe so a slow/hung VM can't stall list/port/inspect.
const wslLookupTimeout = 3 * time.Second

// ListWSLPortOwners returns a map from host port number to the WSL
// process actually behind it. On Windows, a port published by a process
// running inside WSL shows up locally as owned by wslrelay.exe — a single
// relay shared across every distro, which tells you nothing about which
// one (or which process inside it) is actually involved. This resolves
// that the same way #24 resolved Docker: by asking the thing that
// actually knows (here, `ss` run inside each running distro) rather than
// trying to infer it from the Windows side.
//
// Best-effort, no-op helper: wsl.exe missing, no distros running, or any
// other failure all result in a nil map rather than an error. On
// non-Windows platforms wsl.exe simply isn't on PATH, so this is already
// a free no-op there.
func ListWSLPortOwners(ctx context.Context) map[int]*models.WSLProcess {
	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, wslLookupTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "wsl.exe", "-l", "--running", "-q").Output()
	if err != nil {
		return nil
	}

	result := map[int]*models.WSLProcess{}
	for _, distro := range parseWSLDistroList(out) {
		ssOut, err := exec.CommandContext(ctx, "wsl.exe", "-d", distro, "--", "ss", "-tlnp").Output()
		if err != nil {
			continue
		}
		for port, proc := range parseSSListeners(distro, ssOut) {
			result[port] = proc
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parseWSLDistroList decodes wsl.exe's list output. wsl.exe emits
// UTF-16LE when its stdout isn't a real console (as exec.Command's pipe
// isn't), so raw output can't be treated as UTF-8 directly.
//
// ponytail: byte-stripping decode (drop every zero high byte) rather than
// a full UTF-16 decoder — distro names are plain identifiers in practice,
// so this covers the real cases without a new dependency; a genuinely
// non-ASCII distro name would decode wrong. Upgrade to
// golang.org/x/text/encoding/unicode if that ever matters.
func parseWSLDistroList(b []byte) []string {
	decoded := make([]byte, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if b[i+1] == 0 {
			decoded = append(decoded, b[i])
		}
	}

	var distros []string
	for _, line := range strings.Split(string(decoded), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			distros = append(distros, line)
		}
	}
	return distros
}

// ssListenerRe extracts the process name and PID from one `ss -tlnp` LISTEN
// line's `users:(("name",pid=123,fd=45))` column.
var ssListenerRe = regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)

// parseSSListeners parses `ss -tlnp` output (run inside a WSL distro) into
// a map of host port -> owning process. Lines without a resolvable
// process (e.g. permission denied for a foreign-user socket) are skipped
// rather than guessed at.
func parseSSListeners(distro string, out []byte) map[int]*models.WSLProcess {
	result := map[int]*models.WSLProcess{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "LISTEN") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		localAddr := fields[3]
		colon := strings.LastIndex(localAddr, ":")
		if colon < 0 {
			continue
		}
		port, err := strconv.Atoi(localAddr[colon+1:])
		if err != nil || port == 0 {
			continue
		}

		m := ssListenerRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pid, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}

		result[port] = &models.WSLProcess{Distro: distro, PID: int32(pid), Name: m[1]}
	}
	return result
}
