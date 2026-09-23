package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arpitbhalla1801/localpilot/internal/agent"
	"github.com/arpitbhalla1801/localpilot/internal/models"
	"github.com/arpitbhalla1801/localpilot/internal/output"
	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X ...cli.version=vX.Y.Z".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "localpilot",
	Short: "Your command center for everything running on your machine",
	Long:  "LocalPilot helps you discover, understand, diagnose, and control everything running on localhost.",
	Example: "  localpilot                # dashboard of everything listening on localhost\n" +
		"  localpilot port 3000       # what's using port 3000?\n" +
		"  localpilot kill 3000       # free it up (with confirmation)",
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

// RootCommand exposes the root cobra command for tooling that needs to
// walk the command tree, such as the man page generator in tools/gen-man.
func RootCommand() *cobra.Command {
	return rootCmd
}

// negativeNumberArg and commandsWithBareNumericArg support
// insertDashDashForNegativeArgs below.
var negativeNumberArg = regexp.MustCompile(`^-[0-9]+$`)

var commandsWithBareNumericArg = map[string]bool{
	"port":    true,
	"inspect": true,
	"kill":    true,
	"free":    true,
	"watch":   true,
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
	rootCmd.AddCommand(freeCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(doctorCmd)
}

var listJSON bool
var showAll bool

// nonNilPorts ensures a JSON list endpoint always marshals to "[]" for an
// empty result rather than "null" (Go's encoding/json for a nil slice),
// which a caller expecting an array to iterate would otherwise choke on.
func nonNilPorts(ports []models.Port) []models.Port {
	if ports == nil {
		return []models.Port{}
	}
	return ports
}

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

var listRange string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List running processes and listening ports",
	Example: "  localpilot list\n" +
		"  localpilot list --all               # include system/background processes\n" +
		"  localpilot list --range 3000-4000   # only a known dev-server port band\n" +
		"  localpilot list --json | jq '.[].port'",
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

		if listRange != "" {
			start, end, err := parsePortRange(listRange)
			if err != nil {
				return err
			}
			ports = filterPortRange(ports, start, end)
		}

		if listJSON {
			return printJSON(cmd.OutOrStdout(), nonNilPorts(ports))
		}
		output.PrintList(ports)
		return nil
	},
}

var portJSON bool

var portCmd = &cobra.Command{
	Use:   "port <PORT>",
	Short: "Find what is using a port",
	Example: "  localpilot port 3000\n" +
		"  localpilot port 3000 --json | jq '.process.pid'",
	Args: cobra.ExactArgs(1),
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
			return printJSON(cmd.OutOrStdout(), binding)
		}
		output.PrintPortDoctor(binding)
		return nil
	},
}

var inspectJSON bool

var inspectCmd = &cobra.Command{
	Use:   "inspect <PID>",
	Short: "Inspect a process in detail",
	Example: "  localpilot inspect 12345\n" +
		"  localpilot inspect 12345 --json | jq '.process.command'",
	Args: cobra.ExactArgs(1),
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
			return printJSON(cmd.OutOrStdout(), struct {
				Process interface{} `json:"process"`
				Project interface{} `json:"project"`
			}{proc, project})
		}
		output.PrintInspect(proc, project)
		return nil
	},
}

var killForce bool
var killJSON bool
var killContainer bool

// terminationResult is the --json payload shared by kill and free, so
// scripts piping into jq see the same shape from either command.
type terminationResult struct {
	PID              int32  `json:"pid"`
	Port             *int   `json:"port,omitempty"`
	Terminated       bool   `json:"terminated"`
	Cancelled        bool   `json:"cancelled,omitempty"`
	Container        string `json:"container,omitempty"`
	ContainerStopped bool   `json:"containerStopped,omitempty"`
}

// resolveContainerStop decides whether to stop the Docker container
// publishing a port rather than kill the local process that's merely
// proxying it (killing docker-proxy or Docker Desktop's backend process
// alone typically doesn't free the port — Docker keeps it bound to the
// container). container may be nil, meaning the port/process isn't
// container-owned, in which case this always returns (false, false).
//
// soleContainer reports whether this is the only container currently
// running. When it is, the local process can't be shared with anything
// else — Linux's docker-proxy is one-per-port anyway, and even Docker
// Desktop's single shared backend process has nothing else to affect — so
// killing it is exactly as safe as it was before container-awareness
// existed, and no extra ceremony is needed.
//
// When other containers are also running, that safety guarantee is gone:
// Docker Desktop's backend process is shared across every container's
// port mappings, so killing it could take down unrelated containers, not
// just the one at hand. Interactively, the user is asked which action to
// take. In --force mode there's no one to ask, so this is never guessed:
// blocked is returned true unless --container was passed explicitly,
// and the caller must refuse to proceed rather than silently risk it.
func resolveContainerStop(container *models.Container, proc *models.Process, force, containerFlag, soleContainer bool) (stop, blocked bool) {
	if container == nil {
		return false, false
	}
	if containerFlag {
		return true, false
	}
	if soleContainer {
		return false, false
	}
	if force {
		return false, true
	}
	output.PrintContainerConflict(container, proc)
	fmt.Print("Other containers are also running, so killing the process risks affecting them too.\n")
	fmt.Print("Stop the container instead of killing the process? [Y/n] ")
	var response string
	fmt.Scanln(&response)
	return response == "" || response == "y" || response == "Y", false
}

// soleRunningContainer reports whether c is the only container currently
// running, per RunningContainerCount. If the count can't be determined
// (Docker unreachable), it conservatively reports false so callers treat
// the situation as if other containers might exist.
func soleRunningContainer(ctx context.Context, a *agent.Agent, c *models.Container) bool {
	if c == nil {
		return true
	}
	n, ok := a.RunningContainerCount(ctx)
	return ok && n <= 1
}

var killCmd = &cobra.Command{
	Use:   "kill <PID|PORT>",
	Short: "Safely terminate a process",
	Long: "Safely terminate a process by PID or port.\n\n" +
		"Without --force, this prompts for confirmation (\"Are you sure? [y/N]\")\n" +
		"before killing anything. If stdin is not a terminal, that prompt can\n" +
		"never be answered, so --force is required for non-interactive use\n" +
		"(scripts, cron, CI, agents).\n\n" +
		"--json requires --force: an interactive confirmation prompt would\n" +
		"otherwise corrupt stdout for a script expecting JSON.\n\n" +
		"If the port/PID is owned by a Docker container, killing the local\n" +
		"process (docker-proxy, or Docker Desktop's backend process) usually\n" +
		"won't free the port — Docker keeps it bound to the container. In\n" +
		"that case you're asked whether to stop the container instead; with\n" +
		"--force, pass --container explicitly to stop it (the default\n" +
		"without --container is still to kill the process, unchanged).",
	Example: "  localpilot kill 3000                        # kill by port, with confirmation\n" +
		"  localpilot kill 12345 --force               # kill by PID, no confirmation\n" +
		"  localpilot kill 3000 --force --json\n" +
		"  localpilot kill 3000 --force --container    # stop the owning container instead",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if killJSON && !killForce {
			return fmt.Errorf("--json requires --force")
		}

		a, err := agent.New()
		if err != nil {
			return err
		}

		if !killForce && !isInteractiveStdin() {
			return fmt.Errorf("refusing to prompt for confirmation: stdin is not a terminal; pass --force to kill non-interactively")
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
				stop, blocked := resolveContainerStop(binding.Container, binding.Process, killForce, killContainer, soleRunningContainer(ctx, a, binding.Container))
				if blocked {
					return fmt.Errorf("port %d is owned by Docker container %q, and other containers are also running: pass --container to stop it, since killing the process risks affecting them too", port, binding.Container.Name)
				}
				if stop {
					if err := a.StopContainer(ctx, binding.Container.ID); err != nil {
						return fmt.Errorf("failed to stop container %s: %w", binding.Container.Name, err)
					}
					if killJSON {
						return printJSON(cmd.OutOrStdout(), terminationResult{Port: &port, Terminated: true, Container: binding.Container.Name, ContainerStopped: true})
					}
					fmt.Printf("Container %s (port %d) stopped.\n", binding.Container.Name, port)
					return nil
				}

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
				if killJSON {
					return printJSON(cmd.OutOrStdout(), terminationResult{PID: binding.Process.PID, Port: &port, Terminated: true})
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

		stop, blocked := resolveContainerStop(proc.Container, proc, killForce, killContainer, soleRunningContainer(ctx, a, proc.Container))
		if blocked {
			return fmt.Errorf("process %d is owned by Docker container %q, and other containers are also running: pass --container to stop it, since killing the process risks affecting them too", pid, proc.Container.Name)
		}
		if stop {
			if err := a.StopContainer(ctx, proc.Container.ID); err != nil {
				return fmt.Errorf("failed to stop container %s: %w", proc.Container.Name, err)
			}
			if killJSON {
				return printJSON(cmd.OutOrStdout(), terminationResult{Terminated: true, Container: proc.Container.Name, ContainerStopped: true})
			}
			fmt.Printf("Container %s stopped.\n", proc.Container.Name)
			return nil
		}

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
		if killJSON {
			return printJSON(cmd.OutOrStdout(), terminationResult{PID: pid, Terminated: true})
		}
		fmt.Printf("Process %d terminated.\n", pid)
		return nil
	},
}

var freeForce bool
var freeJSON bool
var freeContainer bool

var freeCmd = &cobra.Command{
	Use:   "free <PORT>",
	Short: "Kill whatever is listening on a port",
	Long: "Free a port by killing the process listening on it.\n\n" +
		"Unlike `kill`, the argument is always interpreted as a port, never a\n" +
		"PID, so there is no ambiguity between the two.\n\n" +
		"Without --force, this prompts for confirmation (\"Are you sure? [y/N]\")\n" +
		"before killing anything. If stdin is not a terminal, that prompt can\n" +
		"never be answered, so --force is required for non-interactive use\n" +
		"(scripts, cron, CI, agents).\n\n" +
		"--json requires --force: an interactive confirmation prompt would\n" +
		"otherwise corrupt stdout for a script expecting JSON.\n\n" +
		"If the port is owned by a Docker container, killing the local\n" +
		"process (docker-proxy, or Docker Desktop's backend process) usually\n" +
		"won't free the port — Docker keeps it bound to the container. In\n" +
		"that case you're asked whether to stop the container instead; with\n" +
		"--force, pass --container explicitly to stop it (the default\n" +
		"without --container is still to kill the process, unchanged).",
	Example: "  localpilot free 3000                        # a dev server left the port bound; free it\n" +
		"  localpilot free 3000 --force --json\n" +
		"  localpilot free 3000 --force --container    # stop the owning container instead",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if freeJSON && !freeForce {
			return fmt.Errorf("--json requires --force")
		}

		port, err := parsePort(args[0])
		if err != nil {
			return err
		}

		a, err := agent.New()
		if err != nil {
			return err
		}

		if !freeForce && !isInteractiveStdin() {
			return fmt.Errorf("refusing to prompt for confirmation: stdin is not a terminal; pass --force to free non-interactively")
		}

		ctx := context.Background()
		binding, err := a.FindPort(ctx, port)
		if err != nil {
			return err
		}

		if !binding.InUse || binding.Process == nil {
			if freeJSON {
				return printJSON(cmd.OutOrStdout(), terminationResult{Port: &port, Terminated: false})
			}
			fmt.Printf("Port %d is not in use.\n", port)
			return nil
		}

		stop, blocked := resolveContainerStop(binding.Container, binding.Process, freeForce, freeContainer, soleRunningContainer(ctx, a, binding.Container))
		if blocked {
			return fmt.Errorf("port %d is owned by Docker container %q, and other containers are also running: pass --container to stop it, since killing the process risks affecting them too", port, binding.Container.Name)
		}
		if stop {
			if err := a.StopContainer(ctx, binding.Container.ID); err != nil {
				return fmt.Errorf("failed to stop container %s: %w", binding.Container.Name, err)
			}
			if freeJSON {
				return printJSON(cmd.OutOrStdout(), terminationResult{Port: &port, Terminated: true, Container: binding.Container.Name, ContainerStopped: true})
			}
			fmt.Printf("Container %s (port %d) stopped.\n", binding.Container.Name, port)
			return nil
		}

		if !freeForce {
			output.PrintKillConfirmation(binding)
			if !confirm() {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := a.Kill(ctx, binding.Process.PID, freeForce); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", binding.Process.PID, err)
		}
		if freeJSON {
			return printJSON(cmd.OutOrStdout(), terminationResult{PID: binding.Process.PID, Port: &port, Terminated: true})
		}
		fmt.Printf("Process %d (port %d) terminated.\n", binding.Process.PID, port)
		return nil
	},
}

var watchInterval time.Duration

// resolveWatchTarget mirrors killCmd's PID/port disambiguation: a bare
// number is ambiguous between the two, and a port match always wins
// because `watch <target>` is documented primarily for ports (see README).
func resolveWatchTarget(ctx context.Context, a *agent.Agent, target string) (port int, pid int32, isPort bool, err error) {
	if p, portErr := parsePort(target); portErr == nil {
		binding, findErr := a.FindPort(ctx, p)
		if findErr == nil && binding.InUse && binding.Process != nil {
			return p, 0, true, nil
		}
	}
	if id, pidErr := parsePID(target); pidErr == nil {
		return 0, id, false, nil
	}
	return 0, 0, false, fmt.Errorf("invalid target: must be a PID or port number")
}

// watchTick renders one snapshot for the resolved target. It never returns
// an error for "process is gone" — that's an expected, displayable state
// for a flapping service, which is the whole point of `watch`.
func watchTick(ctx context.Context, a *agent.Agent, port int, pid int32, isPort bool) {
	fmt.Print("\x1b[H\x1b[2J")
	fmt.Printf("Every %s — Ctrl+C to exit — %s\n\n", watchInterval, time.Now().Format("15:04:05"))

	if isPort {
		binding, err := a.FindPort(ctx, port)
		if err != nil {
			fmt.Printf("Error checking port %d: %v\n", port, err)
			return
		}
		output.PrintPortDoctor(binding)
		return
	}

	proc, err := a.InspectProcess(ctx, pid)
	if err != nil {
		fmt.Printf("Process %d is not running.\n", pid)
		return
	}
	project := agent.DetectProject(proc.Cwd)
	output.PrintInspect(proc, project)
}

// runWatch drives the refresh loop until ctx is cancelled (e.g. Ctrl+C).
func runWatch(ctx context.Context, a *agent.Agent, target string) error {
	port, pid, isPort, err := resolveWatchTarget(ctx, a, target)
	if err != nil {
		return err
	}

	watchTick(ctx, a, port, pid, isPort)

	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			watchTick(ctx, a, port, pid, isPort)
		}
	}
}

var watchCmd = &cobra.Command{
	Use:   "watch <PID|PORT>",
	Short: "Live-updating view of a port or process",
	Long: "Live-updating view of a port or process, refreshed on an interval\n" +
		"until interrupted with Ctrl+C.\n\n" +
		"Useful for debugging a service that flaps or restarts, without\n" +
		"re-running `port`/`inspect` manually in a loop.",
	Example: "  localpilot watch 3000\n" +
		"  localpilot watch 12345 --interval 500ms",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if watchInterval <= 0 {
			return fmt.Errorf("invalid --interval: must be greater than 0")
		}

		a, err := agent.New()
		if err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		return runWatch(ctx, a, args[0])
	},
}

var doctorJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Scan for port conflicts and misconfigurations",
	Long: "Scan listening ports for port-related problems:\n\n" +
		"  - multiple processes bound to the same port\n" +
		"  - different projects configured (via .env/package.json) for the\n" +
		"    same default port\n\n" +
		"Scoped to port conflicts only; not a general system health check.",
	Example: "  localpilot doctor\n" +
		"  localpilot doctor --json | jq '.[].message'",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := agent.New()
		if err != nil {
			return err
		}

		ports, err := a.ListPorts(context.Background())
		if err != nil {
			return err
		}

		conflicts := agent.DetectConflicts(filterPorts(ports))

		if doctorJSON {
			return printJSON(cmd.OutOrStdout(), nonNilConflicts(conflicts))
		}
		output.PrintDoctor(conflicts)
		return nil
	},
}

// nonNilConflicts mirrors nonNilPorts: an empty result marshals to "[]"
// rather than "null" so a script piping into jq gets an iterable array.
func nonNilConflicts(conflicts []models.Conflict) []models.Conflict {
	if conflicts == nil {
		return []models.Conflict{}
	}
	return conflicts
}

func init() {
	rootCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all processes (including system/background)")
	listCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all processes (including system/background)")
	killCmd.Flags().BoolVar(&killForce, "force", false, "Skip confirmation and force kill")
	killCmd.Flags().BoolVar(&killJSON, "json", false, "Output as JSON (requires --force)")
	killCmd.Flags().BoolVar(&killContainer, "container", false, "With --force, stop the owning Docker container instead of killing the process")
	freeCmd.Flags().BoolVar(&freeForce, "force", false, "Skip confirmation and force kill")
	freeCmd.Flags().BoolVar(&freeJSON, "json", false, "Output as JSON (requires --force)")
	freeCmd.Flags().BoolVar(&freeContainer, "container", false, "With --force, stop the owning Docker container instead of killing the process")
	watchCmd.Flags().DurationVar(&watchInterval, "interval", time.Second, "Refresh interval (e.g. 1s, 500ms)")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	listCmd.Flags().StringVar(&listRange, "range", "", "Only show ports within <start>-<end>, e.g. 3000-4000")
	portCmd.Flags().BoolVar(&portJSON, "json", false, "Output as JSON")
	inspectCmd.Flags().BoolVar(&inspectJSON, "json", false, "Output as JSON")
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output as JSON")
}

// printJSON writes to w rather than os.Stdout directly so JSON output is
// captured correctly by cmd.SetOut in tests, and honors any output
// redirection a caller sets up.
func printJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// parsePortRange parses a "<start>-<end>" string into its bounds, e.g.
// "3000-4000". Both bounds must be valid ports and start must not exceed
// end.
func parsePortRange(s string) (start, end int, err error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range %q: expected format <start>-<end>, e.g. 3000-4000", s)
	}
	start, err = parsePort(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range %q: %w", s, err)
	}
	end, err = parsePort(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range %q: %w", s, err)
	}
	if start > end {
		return 0, 0, fmt.Errorf("invalid range %q: start must not exceed end", s)
	}
	return start, end, nil
}

func filterPortRange(ports []models.Port, start, end int) []models.Port {
	var filtered []models.Port
	for _, p := range ports {
		if p.Number >= start && p.Number <= end {
			filtered = append(filtered, p)
		}
	}
	return filtered
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
	if err != nil || pid < 1 || pid > math.MaxInt32 {
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

// isInteractiveStdin reports whether stdin is attached to a terminal,
// i.e. whether a confirmation prompt could ever actually be answered.
// Note this is not the same as stdin being a character device: /dev/null
// (the common case for cron/CI/agents launched without a controlling
// terminal) is itself a character device, so that check alone isn't
// enough to catch it.
func isInteractiveStdin() bool {
	return isTerminal(int(os.Stdin.Fd()))
}
