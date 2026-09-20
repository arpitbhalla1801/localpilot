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
	return a.provider.FindPort(ctx, port)
}

// InspectProcess returns detailed information about a process.
func (a *Agent) InspectProcess(ctx context.Context, pid int32) (*models.Process, error) {
	return a.provider.GetProcess(ctx, pid)
}

// ListPorts returns all listening ports on the system.
func (a *Agent) ListPorts(ctx context.Context) ([]models.Port, error) {
	return a.provider.ListListeningPorts(ctx)
}

// Kill terminates a process by PID.
func (a *Agent) Kill(ctx context.Context, pid int32, force bool) error {
	return a.provider.KillProcess(ctx, pid, force)
}

// DetectProject exposes project detection for a working directory.
func DetectProject(cwd string) *models.Project {
	return platform.DetectProject(cwd)
}
