package platform

import (
	"context"

	"github.com/localpilot/localpilot/internal/models"
)

// ProcessProvider abstracts OS-specific process and network inspection.
type ProcessProvider interface {
	GetProcess(ctx context.Context, pid int32) (*models.Process, error)
	ListListeningPorts(ctx context.Context) ([]models.Port, error)
	FindPort(ctx context.Context, port int) (*models.PortBinding, error)
	KillProcess(ctx context.Context, pid int32, force bool) error
}
