package platform

import (
	"context"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// LinuxProvider implements ProcessProvider for Linux systems.
type LinuxProvider struct{}

func (p *LinuxProvider) GetProcess(ctx context.Context, pid int32) (*models.Process, error) {
	proc, err := enrichProcess(ctx, pid)
	if err != nil {
		return nil, err
	}
	return proc, nil
}

func (p *LinuxProvider) ListListeningPorts(ctx context.Context) ([]models.Port, error) {
	return listAllListeningPorts(ctx)
}

func (p *LinuxProvider) FindPort(ctx context.Context, port int) (*models.PortBinding, error) {
	return findListeningPort(ctx, port)
}

func (p *LinuxProvider) KillProcess(ctx context.Context, pid int32, force bool) error {
	return killPID(ctx, pid, force)
}
