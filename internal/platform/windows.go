package platform

import (
	"context"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// WindowsProvider implements ProcessProvider for Windows systems.
type WindowsProvider struct{}

func (p *WindowsProvider) GetProcess(ctx context.Context, pid int32) (*models.Process, error) {
	proc, err := enrichProcess(ctx, pid)
	if err != nil {
		return nil, err
	}
	return proc, nil
}

func (p *WindowsProvider) ListListeningPorts(ctx context.Context) ([]models.Port, error) {
	return listAllListeningPorts(ctx)
}

func (p *WindowsProvider) FindPort(ctx context.Context, port int) (*models.PortBinding, error) {
	return findListeningPort(ctx, port)
}

func (p *WindowsProvider) KillProcess(ctx context.Context, pid int32, force bool) error {
	return killPID(ctx, pid, force)
}
