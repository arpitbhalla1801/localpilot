package cli

import (
	"os"
	"testing"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// withStdin temporarily replaces os.Stdin with a pipe pre-loaded with
// input, for exercising confirm()-based prompts without blocking on a
// real terminal.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("write to pipe: %v", err)
	}
	w.Close()

	original := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = original
		r.Close()
	})
}

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

// TestResolveContainerStop_ExplicitFlagWithoutForceStillConfirms is the
// regression test for #33: --container without --force must still ask
// for confirmation like every other kill/free path, rather than stopping
// the container immediately just because the flag was passed.
func TestResolveContainerStop_ExplicitFlagWithoutForceStillConfirms(t *testing.T) {
	container := &models.Container{ID: "abc123", Name: "my-app", Image: "nginx:alpine"}
	proc := &models.Process{PID: 1}

	t.Run("declining the prompt does not stop the container", func(t *testing.T) {
		withStdin(t, "n\n")
		stop, blocked := resolveContainerStop(container, proc, false /* force */, true /* containerFlag */, false /* soleContainer */)
		if stop || blocked {
			t.Errorf("resolveContainerStop(containerFlag=true, force=false, answer=n) = (stop=%v, blocked=%v), want (false, false)", stop, blocked)
		}
	})

	t.Run("confirming the prompt stops the container", func(t *testing.T) {
		withStdin(t, "y\n")
		stop, blocked := resolveContainerStop(container, proc, false /* force */, true /* containerFlag */, false /* soleContainer */)
		if !stop || blocked {
			t.Errorf("resolveContainerStop(containerFlag=true, force=false, answer=y) = (stop=%v, blocked=%v), want (true, false)", stop, blocked)
		}
	})
}
