package cli

import (
	"testing"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// TestResolveContainerStop_Force covers the non-interactive branches,
// which are fully deterministic:
//   - no container in play: always a no-op, regardless of flags.
//   - this is the only container running: safe either way, so killing
//     the process proceeds without needing --container (pre-#25 default
//     behavior, restored for the case where nothing else can be
//     affected).
//   - other containers are also running: never guessed. --container
//     stops the container; without it, force mode refuses to guess and
//     reports blocked=true rather than risk killing a process shared
//     with unrelated containers.
func TestResolveContainerStop_Force(t *testing.T) {
	container := &models.Container{ID: "abc123", Name: "my-app", Image: "nginx:alpine"}
	proc := &models.Process{PID: 100, Name: "docker-proxy"}

	tests := []struct {
		name          string
		container     *models.Container
		containerFlag bool
		soleContainer bool
		wantStop      bool
		wantBlocked   bool
	}{
		{"no container present, flag off", nil, false, false, false, false},
		{"no container present, flag on is still a no-op", nil, true, false, false, false},
		{"sole container, flag off kills the process safely", container, false, true, false, false},
		{"sole container, flag on still stops the container", container, true, true, true, false},
		{"other containers running, flag off is blocked rather than guessed", container, false, false, false, true},
		{"other containers running, flag on stops the container", container, true, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stop, blocked := resolveContainerStop(tt.container, proc, true /* force */, tt.containerFlag, tt.soleContainer)
			if stop != tt.wantStop || blocked != tt.wantBlocked {
				t.Errorf("resolveContainerStop(force=true, container=%v, sole=%v) = (stop=%v, blocked=%v), want (stop=%v, blocked=%v)",
					tt.containerFlag, tt.soleContainer, stop, blocked, tt.wantStop, tt.wantBlocked)
			}
		})
	}
}

func TestResolveContainerStop_NilContainerNeverPrompts(t *testing.T) {
	// With no container, the function must short-circuit before touching
	// stdin/stdout at all — interactively it would otherwise block this
	// test on a Scanln read that never resolves.
	stop, blocked := resolveContainerStop(nil, &models.Process{PID: 1}, false /* force */, false, false)
	if stop || blocked {
		t.Errorf("resolveContainerStop with nil container = (stop=%v, blocked=%v), want (false, false)", stop, blocked)
	}
}

func TestResolveContainerStop_SoleContainerNeverPromptsInteractively(t *testing.T) {
	// Being the only running container makes this exactly as safe as a
	// non-container kill always was, interactively too — it must not
	// block on stdin here either.
	container := &models.Container{ID: "abc123", Name: "my-app", Image: "nginx:alpine"}
	stop, blocked := resolveContainerStop(container, &models.Process{PID: 1}, false /* force */, false, true /* soleContainer */)
	if stop || blocked {
		t.Errorf("resolveContainerStop for the sole running container = (stop=%v, blocked=%v), want (false, false)", stop, blocked)
	}
}
