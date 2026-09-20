// Command gen-docs generates Markdown command reference pages for the docs
// site into docs/content/commands, using cobra's doc generator to walk the
// real command tree (mirrors tools/gen-man, which does the same for man
// pages) so the docs never drift from --help text.
//
// Run via `go run ./tools/gen-docs`; wired into the docs GitHub Actions
// workflow so the site rebuilds from the current command tree on every push.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arpitbhalla1801/localpilot/internal/cli"
	"github.com/spf13/cobra/doc"
)

func main() {
	outDir := "docs/content/commands"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	if err := os.RemoveAll(outDir); err != nil {
		fail(err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail(err)
	}

	root := cli.RootCommand()

	linkHandler := func(name string) string {
		base := strings.TrimSuffix(name, ".md")
		if base == "localpilot" {
			return "/commands/"
		}
		return "/commands/" + base + "/"
	}

	filePrepender := func(filename string) string {
		base := strings.TrimSuffix(filepath.Base(filename), ".md")
		title := strings.ReplaceAll(base, "_", " ")
		return fmt.Sprintf("---\ntitle: \"%s\"\nweight: 10\n---\n\n", title)
	}

	if err := doc.GenMarkdownTreeCustom(root, outDir, filePrepender, linkHandler); err != nil {
		fail(err)
	}

	// GenMarkdownTreeCustom writes one file per command including the root
	// (localpilot.md); rename it to _index.md so Hugo Book treats the
	// command tree as a section landing page instead of a leaf.
	rootFile := filepath.Join(outDir, "localpilot.md")
	if b, err := os.ReadFile(rootFile); err == nil {
		b = bytes.Replace(b, []byte(`title: "localpilot"`), []byte(`title: "Commands"`), 1)
		if err := os.WriteFile(filepath.Join(outDir, "_index.md"), b, 0o644); err != nil {
			fail(err)
		}
		os.Remove(rootFile)
	}

	fmt.Printf("generated command docs in %s\n", outDir)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-docs:", err)
	os.Exit(1)
}
