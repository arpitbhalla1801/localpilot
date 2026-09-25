package agent

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

func TestMarkStale_SkipsYoungSystemAndUnknown(t *testing.T) {
	// The test process is idle enough to be marked if it reached
	// sampling; every port below must be filtered out before that.
	pid := int32(os.Getpid())

	old := time.Now().Add(-72 * time.Hour)
	ports := []models.Port{
		{Number: 1, Process: &models.Process{PID: pid, Name: "young", StartTime: time.Now()}},
		{Number: 2, Process: &models.Process{PID: pid, Name: "svchost.exe", StartTime: old}},
		{Number: 3, Process: &models.Process{PID: pid, Name: "no-start-time"}},
		{Number: 4},
	}
	(&Agent{}).MarkStale(context.Background(), ports, 48*time.Hour)
	for _, p := range ports {
		if p.Stale {
			t.Errorf("port %d marked stale, want not stale", p.Number)
		}
	}
}
