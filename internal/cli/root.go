package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/localpilot/localpilot/internal/agent"
	"github.com/localpilot/localpilot/internal/models"
	"github.com/localpilot/localpilot/internal/output"
	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X ...cli.version=vX.Y.Z".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:           "localpilot",
	Short:         "Your command center for everything running on your machine",
	Long:          "LocalPilot helps you discover, understand, diagnose, and control everything running on localhost.",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := agent.New()
		if err != nil {
			return err
		}

		ports, err := a.ListPorts(context.Background())
		if err != nil {
			return err
		}

		ports = filterPorts(ports)

		output.PrintDashboard(ports)
		return nil
	},
}

// Execute runs the root command.
func Execute() error {
	rootCmd.SetArgs(insertDashDashForNegativeArgs(os.Args[1:]))
	return rootCmd.Execute()
}

// negativeNumberArg and commandsWithBareNumericArg support
// insertDashDashForNegativeArgs below.
var negativeNumberArg = regexp.MustCompile(`^-[0-9]+$`)

var commandsWithBareNumericArg = map[string]bool{
	"port":    true,
	"inspect": true,
	"kill":    true,
}

// insertDashDashForNegativeArgs rewrites e.g. "inspect -5" to
// "inspect -- -5". Without this, pflag treats a leading "-5" as an
// (unknown) shorthand flag cluster rather than the command's
// positional PID/port argument, producing a confusing "unknown
// shorthand flag" error instead of our own "invalid PID"/"invalid
// port" message.
func insertDashDashForNegativeArgs(args []string) []string {
	for i, a := range args {
		if a == "--" {
			return args
		}
		if !commandsWithBareNumericArg[a] {
			continue
		}
		if i+1 < len(args) && negativeNumberArg.MatchString(args[i+1]) {
			out := make([]string, 0, len(args)+1)
			out = append(out, args[:i+1]...)
			out = append(out, "--")
			out = append(out, args[i+1:]...)
			return out
		}
	}
	return args
}

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(portCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(killCmd)
}

var listJSON bool
var showAll bool

func filterPorts(ports []models.Port) []models.Port {
	if showAll {
		return ports
	}
	var filtered []models.Port
	for _, p := range ports {
		// Hide processes we identify as system/background noise
		if p.Process != nil && output.IsSystemOrBackgroundProcess(p.Process.Name) {
			continue
		}
		// Also hide completely empty/restricted ports where no process info was found
		if p.Process != nil && p.Process.Name == "Restricted" {
			continue
		}
		filtered = append(filtered, p)
	}
	return filtered
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List running processes and listening ports",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := agent.New()
		if err != nil {
			return err
		}

		ports, err := a.ListPorts(context.Background())
		if err != nil {
			return err
		}

		ports = filterPorts(ports)

		if listJSON {
			return printJSON(ports)
		}
		output.PrintList(ports)
		return nil
	},
}

var portJSON bool

var portCmd = &cobra.Command{
	Use:   "port <PORT>",
	Short: "Find what is using a port",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := parsePort(args[0])
		if err != nil {
			return err
		}

		a, err := agent.New()
		if err != nil {
			return err
		}

		binding, err := a.FindPort(context.Background(), port)
		if err != nil {
			return err
		}

		if portJSON {
			return printJSON(binding)
		}
		output.PrintPortDoctor(binding)
		return nil
	},
}

var inspectJSON bool

var inspectCmd = &cobra.Command{
	Use:   "inspect <PID>",
	Short: "Inspect a process in detail",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pid, err := parsePID(args[0])
		if err != nil {
			return err
		}

		a, err := agent.New()
		if err != nil {
			return err
		}

		proc, err := a.InspectProcess(context.Background(), pid)
		if err != nil {
			return err
		}

		project := agent.DetectProject(proc.Cwd)

		if inspectJSON {
			return printJSON(struct {
				Process interface{} `json:"process"`
				Project interface{} `json:"project"`
			}{proc, project})
		}
		output.PrintInspect(proc, project)
		return nil
	},
}

var killForce bool

var killCmd = &cobra.Command{
	Use:   "kill <PID|PORT>",
	Short: "Safely terminate a process",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := agent.New()
		if err != nil {
			return err
		}

		target := args[0]
		ctx := context.Background()

		// PID and port ranges overlap on Linux (pid_max is commonly well
		// under 65535), so a bare number like "3000" is ambiguous. Ports are
		// the documented, primary use of `kill <target>` (see README), and
		// resolving to the wrong PID here means killing an unrelated
		// process — so a port match always wins over a PID match.
		if port, portErr := parsePort(target); portErr == nil {
			binding, err := a.FindPort(ctx, port)
			if err != nil {
				return err
			}
			if binding.InUse && binding.Process != nil {
				if !killForce {
					output.PrintKillConfirmation(binding)
					if !confirm() {
						fmt.Println("Cancelled.")
						return nil
					}
				}

				if err := a.Kill(ctx, binding.Process.PID, killForce); err != nil {
					return fmt.Errorf("failed to kill process %d: %w", binding.Process.PID, err)
				}
				fmt.Printf("Process %d (port %d) terminated.\n", binding.Process.PID, port)
				return nil
			}
		}

		pid, pidErr := parsePID(target)
		if pidErr != nil {
			return fmt.Errorf("invalid target: must be a PID or port number")
		}

		proc, err := a.InspectProcess(ctx, pid)
		if err != nil {
			return fmt.Errorf("no listening port and no process found for %q", target)
		}
		project := agent.DetectProject(proc.Cwd)

		if !killForce {
			output.PrintKillByPIDConfirmation(proc, project)
			if !confirm() {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := a.Kill(ctx, pid, killForce); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", pid, err)
		}
		fmt.Printf("Process %d terminated.\n", pid)
		return nil
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all processes (including system/background)")
	listCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all processes (including system/background)")
	killCmd.Flags().BoolVar(&killForce, "force", false, "Skip confirmation and force kill")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	portCmd.Flags().BoolVar(&portJSON, "json", false, "Output as JSON")
	inspectCmd.Flags().BoolVar(&inspectJSON, "json", false, "Output as JSON")
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func parsePort(s string) (int, error) {
	if s == "" || s[0] == '+' {
		return 0, fmt.Errorf("invalid port: %s", s)
	}
	port, err := strconv.Atoi(s)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port: %s", s)
	}
	return port, nil
}

func parsePID(s string) (int32, error) {
	if s == "" || s[0] == '+' {
		return 0, fmt.Errorf("invalid PID: %s", s)
	}
	pid, err := strconv.Atoi(s)
	if err != nil || pid < 1 {
		return 0, fmt.Errorf("invalid PID: %s", s)
	}
	return int32(pid), nil
}

func confirm() bool {
	fmt.Print("Are you sure? [y/N] ")
	var response string
	fmt.Scanln(&response)
	return response == "y" || response == "Y"
}
