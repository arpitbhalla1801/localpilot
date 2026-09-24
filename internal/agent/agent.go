package agent

import (
	"context"
	"fmt"
	"runtime"

	"github.com/arpitbhalla1801/localpilot/internal/models"
	"github.com/arpitbhalla1801/localpilot/internal/platform"
)

// Agent orchestrates system inspection across platform helpers.
type Agent struct{}

// New creates an Agent, failing on platforms process inspection does not support.
func New() (*Agent, error) {
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		return &Agent{}, nil
	}
	return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
}

// FindPort returns details about what is using a given port.
func (a *Agent) FindPort(ctx context.Context, port int) (*models.PortBinding, error) {
	binding, err := platform.FindListeningPort(ctx, port)
	if err != nil {
		return nil, err
	}
	if binding.InUse {
		binding.Container = platform.ListDockerContainerPorts(ctx)[binding.Port]
		binding.WSL = platform.ListWSLPortOwners(ctx)[binding.Port]
	}
	return binding, nil
}

// firstOwner returns the first of ports that has an entry in owners, or the
// zero value. Reading a nil owners map is safe, so callers need no nil check.
func firstOwner[T any](ports []int, owners map[int]T) (T, bool) {
	for _, p := range ports {
		if o, ok := owners[p]; ok {
			return o, true
		}
	}
	var zero T
	return zero, false
}

// InspectProcess returns detailed information about a process, including
// the Docker container or WSL process behind one of its open ports, if any.
func (a *Agent) InspectProcess(ctx context.Context, pid int32) (*models.Process, error) {
	proc, err := platform.EnrichProcess(ctx, pid)
	if err != nil {
		return nil, err
	}
	if len(proc.OpenPorts) > 0 {
		proc.Container, _ = firstOwner(proc.OpenPorts, platform.ListDockerContainerPorts(ctx))
		proc.WSL, _ = firstOwner(proc.OpenPorts, platform.ListWSLPortOwners(ctx))
	}
	return proc, nil
}

// ListPorts returns all listening ports on the system, with any Docker
// container or WSL process publishing a given port resolved and attached.
func (a *Agent) ListPorts(ctx context.Context) ([]models.Port, error) {
	ports, err := platform.ListAllListeningPorts(ctx)
	if err != nil {
		return nil, err
	}
	containers := platform.ListDockerContainerPorts(ctx)
	wsl := platform.ListWSLPortOwners(ctx)
	for i := range ports {
		ports[i].Container = containers[ports[i].Number]
		ports[i].WSL = wsl[ports[i].Number]
	}
	return ports, nil
}
