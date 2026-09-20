package platform

import (
	"context"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// MacOSProvider implements ProcessProvider for macOS systems.
type MacOSProvider struct{}

func (p *MacOSProvider) GetProcess(ctx context.Context, pid int32) (*models.Process, error) {
	proc, err := enrichProcess(ctx, pid)
	if err != nil {
		return nil, err
	}
	return proc, nil
}

func (p *MacOSProvider) ListListeningPorts(ctx context.Context) ([]models.Port, error) {
	return listAllListeningPorts(ctx)
}

func (p *MacOSProvider) FindPort(ctx context.Context, port int) (*models.PortBinding, error) {
	return findListeningPort(ctx, port)
}

func (p *MacOSProvider) KillProcess(ctx context.Context, pid int32, force bool) error {
	return killPID(ctx, pid, force)
}
