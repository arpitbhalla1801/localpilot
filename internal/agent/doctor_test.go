package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

func TestDetectConflicts_NoConflicts(t *testing.T) {
	ports := []models.Port{
		{Number: 3000, Process: &models.Process{PID: 100}},
		{Number: 4000, Process: &models.Process{PID: 200}},
	}
	got := DetectConflicts(ports)
	if len(got) != 0 {
		t.Fatalf("expected no conflicts, got %+v", got)
	}
}

func TestDetectConflicts_MultipleListenersOnSamePort(t *testing.T) {
	ports := []models.Port{
		{Number: 3000, Address: "0.0.0.0", Process: &models.Process{PID: 100}},
		{Number: 3000, Address: "127.0.0.1", Process: &models.Process{PID: 200}},
	}
	got := DetectConflicts(ports)
	if len(got) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %+v", len(got), got)
	}
	c := got[0]
	if c.Kind != ConflictMultipleListeners {
		t.Errorf("kind = %q, want %q", c.Kind, ConflictMultipleListeners)
	}
	if c.Port != 3000 {
		t.Errorf("port = %d, want 3000", c.Port)
	}
	if len(c.Ports) != 2 {
		t.Errorf("expected 2 ports in conflict, got %d", len(c.Ports))
	}
}

func TestDetectConflicts_SamePIDDifferentAddressIsNotAConflict(t *testing.T) {
	// The same process listening on two addresses for the same port
	// (e.g. IPv4 and IPv6) is normal, not a conflict.
	ports := []models.Port{
		{Number: 3000, Address: "0.0.0.0", Process: &models.Process{PID: 100}},
		{Number: 3000, Address: "::", Process: &models.Process{PID: 100}},
	}
	got := DetectConflicts(ports)
	if len(got) != 0 {
		t.Fatalf("expected no conflicts, got %+v", got)
	}
}

func TestDetectConflicts_SameDefaultPortFromEnv(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	writeFile(t, filepath.Join(dirA, ".env"), "PORT=3000\n")
	writeFile(t, filepath.Join(dirB, ".env"), "PORT=3000\n")

	ports := []models.Port{
		{Number: 3000, Process: &models.Process{PID: 100, Cwd: dirA}},
		{Number: 5173, Process: &models.Process{PID: 200, Cwd: dirB}},
	}

	got := DetectConflicts(ports)
	if len(got) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %+v", len(got), got)
	}
	c := got[0]
	if c.Kind != ConflictSameDefaultPort {
		t.Errorf("kind = %q, want %q", c.Kind, ConflictSameDefaultPort)
	}
	if c.Port != 3000 {
		t.Errorf("port = %d, want 3000", c.Port)
	}
	if len(c.Ports) != 2 {
		t.Errorf("expected 2 ports in conflict, got %d", len(c.Ports))
	}
}

func TestDetectConflicts_SameDefaultPortFromPackageJSON(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	writeFile(t, filepath.Join(dirA, "package.json"), `{"scripts":{"dev":"vite --port 3000"}}`)
	writeFile(t, filepath.Join(dirB, "package.json"), `{"scripts":{"dev":"next dev --port=3000"}}`)

	ports := []models.Port{
		{Number: 3000, Process: &models.Process{PID: 100, Cwd: dirA}},
		{Number: 3001, Process: &models.Process{PID: 200, Cwd: dirB}},
	}

	got := DetectConflicts(ports)
	if len(got) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %+v", len(got), got)
	}
	if got[0].Kind != ConflictSameDefaultPort {
		t.Errorf("kind = %q, want %q", got[0].Kind, ConflictSameDefaultPort)
	}
}

func TestDetectConflicts_DifferentDeclaredPortsIsNotAConflict(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	writeFile(t, filepath.Join(dirA, ".env"), "PORT=3000\n")
	writeFile(t, filepath.Join(dirB, ".env"), "PORT=4000\n")

	ports := []models.Port{
		{Number: 3000, Process: &models.Process{PID: 100, Cwd: dirA}},
		{Number: 4000, Process: &models.Process{PID: 200, Cwd: dirB}},
	}

	got := DetectConflicts(ports)
	if len(got) != 0 {
		t.Fatalf("expected no conflicts, got %+v", got)
	}
}

func TestDetectConflicts_NoConfigNoDeclaredPort(t *testing.T) {
	dir := t.TempDir()
	ports := []models.Port{
		{Number: 3000, Process: &models.Process{PID: 100, Cwd: dir}},
		{Number: 4000, Process: &models.Process{PID: 200, Cwd: t.TempDir()}},
	}
	got := DetectConflicts(ports)
	if len(got) != 0 {
		t.Fatalf("expected no conflicts, got %+v", got)
	}
}

func TestDetectConflicts_SameCwdCountedOnce(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "PORT=3000\n")

	// Same process/cwd listening on two ports shouldn't self-flag.
	ports := []models.Port{
		{Number: 3000, Process: &models.Process{PID: 100, Cwd: dir}},
		{Number: 3001, Process: &models.Process{PID: 100, Cwd: dir}},
	}
	got := DetectConflicts(ports)
	if len(got) != 0 {
		t.Fatalf("expected no conflicts, got %+v", got)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
