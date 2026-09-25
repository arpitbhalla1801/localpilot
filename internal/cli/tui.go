package cli

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/arpitbhalla1801/localpilot/internal/agent"
	"github.com/arpitbhalla1801/localpilot/internal/models"
	"github.com/arpitbhalla1801/localpilot/internal/platform"
	"github.com/arpitbhalla1801/localpilot/internal/tui"
)

var tuiInterval time.Duration

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive live view of listening ports",
	Long: "Interactive terminal view of listening ports, refreshed live.\n\n" +
		"Keys: j/k or arrows move, enter inspects, x kills the selected\n" +
		"process (with confirmation), esc goes back, q quits.",
	Example: "  localpilot tui\n" +
		"  localpilot tui --all --interval 5s",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !isTerminal(int(os.Stdout.Fd())) {
			return errors.New("tui needs an interactive terminal")
		}
		if tuiInterval <= 0 {
			return errors.New("--interval must be positive")
		}
		a, err := agent.New()
		if err != nil {
			return err
		}
		load := func(ctx context.Context) ([]models.Port, error) {
			ports, err := a.ListPorts(ctx)
			if err != nil {
				return nil, err
			}
			ports = filterPorts(ports)
			a.MarkStale(ctx, ports, agent.DefaultStaleAfter)
			return ports, nil
		}
		kill := func(ctx context.Context, pid int32) error { return platform.KillPID(ctx, pid, false) }
		return tui.Run(load, kill, tuiInterval)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
	tuiCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all processes (including system/background)")
	tuiCmd.Flags().DurationVar(&tuiInterval, "interval", 2*time.Second, "Refresh interval (e.g. 2s, 500ms)")
}
