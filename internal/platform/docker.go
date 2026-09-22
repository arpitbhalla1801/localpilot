package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// dockerLookupTimeout bounds how long we'll wait on the docker CLI. A
// missing daemon (Docker Desktop installed but not running) fails fast,
// but a hung socket shouldn't be able to stall list/port/inspect.
const dockerLookupTimeout = 2 * time.Second

// dockerPSEntry mirrors the fields we need from `docker ps --format json`.
type dockerPSEntry struct {
	ID    string `json:"ID"`
	Image string `json:"Image"`
	Names string `json:"Names"`
	Ports string `json:"Ports"`
}

// ListDockerContainerPorts returns a map from host port number to the
// container publishing it, resolved via `docker ps`. This is deliberately
// port-based rather than matching on the owning process's name: the local
// process that appears to own a published port varies by platform
// (docker-proxy on Linux; Docker Desktop's VM-based proxying on
// macOS/Windows often doesn't expose a per-port process at all), while
// docker ps's own view of its port mappings is authoritative everywhere.
//
// It's a best-effort, no-op helper: the docker CLI missing, the daemon not
// running, or any other failure reaching Docker all result in a nil map
// rather than an error, so callers don't need special-case handling for
// "Docker isn't in play here".
func ListDockerContainerPorts(ctx context.Context) map[int]*models.Container {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, dockerLookupTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "ps", "--format", "{{json .}}").Output()
	if err != nil {
		return nil
	}

	result := map[int]*models.Container{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var entry dockerPSEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		container := containerFromPSEntry(entry)
		for _, port := range parseDockerHostPorts(entry.Ports) {
			result[port] = container
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// containerFromPSEntry builds a models.Container from one `docker ps
// --format json` row. Names can be a comma-separated list when a
// container has multiple network aliases; the first is used as the
// canonical display name, matching what `docker ps`'s table output shows.
func containerFromPSEntry(entry dockerPSEntry) *models.Container {
	return &models.Container{
		ID:    entry.ID,
		Name:  strings.TrimPrefix(strings.SplitN(entry.Names, ",", 2)[0], "/"),
		Image: entry.Image,
	}
}

// StopContainer stops a running container by ID via `docker stop`.
func StopContainer(ctx context.Context, id string) error {
	out, err := exec.CommandContext(ctx, "docker", "stop", id).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("docker stop %s: %s", id, msg)
	}
	return nil
}

// parseDockerHostPorts extracts host port numbers from docker ps's Ports
// column, e.g. "0.0.0.0:3000->3000/tcp, :::3000->3000/tcp, 8080/tcp". The
// last form (no "->") is a container-only exposed port with no host
// mapping, so it's never reachable/listed on the host and is skipped.
func parseDockerHostPorts(ports string) []int {
	var result []int
	if ports == "" {
		return result
	}
	for _, part := range strings.Split(ports, ",") {
		part = strings.TrimSpace(part)
		arrow := strings.Index(part, "->")
		if arrow < 0 {
			continue
		}
		hostSide := part[:arrow]
		colon := strings.LastIndex(hostSide, ":")
		if colon < 0 {
			continue
		}
		port, err := strconv.Atoi(hostSide[colon+1:])
		if err != nil {
			continue
		}
		result = append(result, port)
	}
	return result
}
