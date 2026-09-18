// Command gen-man generates man pages for localpilot into ./man, using
// cobra's doc generator to walk the real command tree (so pages never
// drift from --help text). Run via `go run ./tools/gen-man`; wired into
// the release build (see .goreleaser.yaml) so pages ship in release
// archives.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/localpilot/localpilot/internal/cli"
	"github.com/spf13/cobra/doc"
)

func main() {
	outDir := "man"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "gen-man:", err)
		os.Exit(1)
	}

	header := &doc.GenManHeader{
		Title:   "LOCALPILOT",
		Section: "1",
		Source:  "LocalPilot",
		Manual:  "LocalPilot Manual",
		Date:    ptrTime(time.Now()),
	}

	if err := doc.GenManTree(cli.RootCommand(), header, outDir); err != nil {
		fmt.Fprintln(os.Stderr, "gen-man:", err)
		os.Exit(1)
	}

	fmt.Printf("generated man pages in %s\n", outDir)
}

func ptrTime(t time.Time) *time.Time { return &t }
