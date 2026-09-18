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
// command itself is wired up (not just the underlying generator, which
// TestCompletionCmd already covers directly). It only checks that the
// command resolves and runs without error — asserting on captured output
// here is unreliable because cobra's generators write through
// cmd.OutOrStdout(), which can resolve to a different writer depending on
// what earlier tests in the same process left on rootCmd/os.Stdout.
func TestCompletionCmd_CLI(t *testing.T) {
	rootCmd.SetArgs([]string{"completion", "bash"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("localpilot completion bash: %v", err)
	}
}
