package agent

import (
	"context"

	"github.com/arpitbhalla1801/localpilot/internal/models"
	"github.com/arpitbhalla1801/localpilot/internal/platform"
)

// Agent orchestrates system inspection across platform providers.
type Agent struct {
	provider platform.ProcessProvider
}

// New creates an Agent with the platform-appropriate provider.
func New() (*Agent, error) {
	provider, err := platform.NewProvider()
	if err != nil {
		return nil, err
	}
	return &Agent{provider: provider}, nil
}

// FindPort returns details about what is using a given port.
func (a *Agent) FindPort(ctx context.Context, port int) (*models.PortBinding, error) {
	binding, err := a.provider.FindPort(ctx, port)
	if err != nil {
		return nil, err
	}
	if binding.InUse {
		if containers := platform.ListDockerContainerPorts(ctx); containers != nil {
			binding.Container = containers[binding.Port]
		}
	}
	return binding, nil
}

// InspectProcess returns detailed information about a process, including
// the Docker container publishing one of its open ports, if any.
func (a *Agent) InspectProcess(ctx context.Context, pid int32) (*models.Process, error) {
	proc, err := a.provider.GetProcess(ctx, pid)
	if err != nil {
		return nil, err
	}
	if len(proc.OpenPorts) > 0 {
		if containers := platform.ListDockerContainerPorts(ctx); containers != nil {
			for _, port := range proc.OpenPorts {
				if c, ok := containers[port]; ok {
					proc.Container = c
					break
				}
			}
		}
	}
	return proc, nil
}

// ListPorts returns all listening ports on the system, with any Docker
// container publishing a given port resolved and attached.
func (a *Agent) ListPorts(ctx context.Context) ([]models.Port, error) {
	ports, err := a.provider.ListListeningPorts(ctx)
	if err != nil {
		return nil, err
	}
	if containers := platform.ListDockerContainerPorts(ctx); containers != nil {
		for i := range ports {
			ports[i].Container = containers[ports[i].Number]
		}
	}
	return ports, nil
}

// Kill terminates a process by PID.
func (a *Agent) Kill(ctx context.Context, pid int32, force bool) error {
	return a.provider.KillProcess(ctx, pid, force)
}

// DetectProject exposes project detection for a working directory.
func DetectProject(cwd string) *models.Project {
	return platform.DetectProject(cwd)
}
