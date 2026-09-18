package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra/doc"
)

// TestGenManTree is a smoke test for the man page generator in
// tools/gen-man: it exercises the same doc.GenManTree call against the
// real command tree so a change to a command's Use/Short/Long/Example
// that breaks generation (or a cobra upgrade that does) fails here
// instead of only being discovered at release time.
func TestGenManTree(t *testing.T) {
	dir := t.TempDir()
	header := &doc.GenManHeader{
		Title:   "LOCALPILOT",
		Section: "1",
		Source:  "LocalPilot",
		Manual:  "LocalPilot Manual",
		Date:    timePtr(time.Now()),
	}

	if err := doc.GenManTree(rootCmd, header, dir); err != nil {
		t.Fatalf("GenManTree: %v", err)
	}

	for _, name := range []string{"localpilot.1", "localpilot-list.1", "localpilot-port.1", "localpilot-inspect.1", "localpilot-kill.1", "localpilot-free.1", "localpilot-watch.1"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected man page %s: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("man page %s is empty", name)
		}
	}
}

func timePtr(t time.Time) *time.Time { return &t }
