package cli

import (
	"testing"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// TestResolveContainerStop_Force covers the non-interactive branch, which
// is fully deterministic: with --force, the decision must come from
// --container alone, never guessed, since there's no one to prompt.
func TestResolveContainerStop_Force(t *testing.T) {
	container := &models.Container{ID: "abc123", Name: "my-app", Image: "nginx:alpine"}
	proc := &models.Process{PID: 100, Name: "docker-proxy"}

	tests := []struct {
		name          string
		container     *models.Container
		containerFlag bool
		want          bool
	}{
		{"no container present, flag off", nil, false, false},
		{"no container present, flag on is still a no-op", nil, true, false},
		{"container present, flag off keeps killing the process", container, false, false},
		{"container present, flag on stops the container", container, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveContainerStop(tt.container, proc, true /* force */, tt.containerFlag)
			if got != tt.want {
				t.Errorf("resolveContainerStop(force=true, container=%v) = %v, want %v", tt.containerFlag, got, tt.want)
			}
		})
	}
}

func TestResolveContainerStop_NilContainerNeverPrompts(t *testing.T) {
	// With no container, the function must short-circuit before touching
	// stdin/stdout at all — interactively it would otherwise block this
	// test on a Scanln read that never resolves.
	if got := resolveContainerStop(nil, &models.Process{PID: 1}, false /* force */, false); got {
		t.Errorf("resolveContainerStop with nil container = true, want false")
	}
}
