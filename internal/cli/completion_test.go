package cli

import (
	"bytes"
	"testing"
)

// TestCompletionCmd verifies cobra's built-in `completion` command (which
// root.go relies on rather than reimplementing) still generates a
// non-empty script for each supported shell. This guards against it being
// disabled (CompletionOptions.DisableDefaultCmd) or cobra changing
// defaults out from under us.
func TestCompletionCmd(t *testing.T) {
	generators := map[string]func(w *bytes.Buffer) error{
		"bash":       func(w *bytes.Buffer) error { return rootCmd.GenBashCompletionV2(w, true) },
		"zsh":        func(w *bytes.Buffer) error { return rootCmd.GenZshCompletion(w) },
		"fish":       func(w *bytes.Buffer) error { return rootCmd.GenFishCompletion(w, true) },
		"powershell": func(w *bytes.Buffer) error { return rootCmd.GenPowerShellCompletionWithDesc(w) },
	}

	for shell, gen := range generators {
		t.Run(shell, func(t *testing.T) {
			var out bytes.Buffer
			if err := gen(&out); err != nil {
				t.Fatalf("completion %s: %v", shell, err)
			}
			if out.Len() == 0 {
				t.Fatalf("completion %s produced no output", shell)
			}
		})
	}
}

// TestCompletionCmd_CLI verifies the `localpilot completion <shell>`
// command itself is wired up (not just the underlying generator).
func TestCompletionCmd_CLI(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"completion", "bash"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("localpilot completion bash: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("localpilot completion bash produced no output")
	}
}
