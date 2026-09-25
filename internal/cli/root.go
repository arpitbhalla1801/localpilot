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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/arpitbhalla1801/localpilot/internal/agent"
	"github.com/arpitbhalla1801/localpilot/internal/models"
	"github.com/arpitbhalla1801/localpilot/internal/output"
	"github.com/arpitbhalla1801/localpilot/internal/platform"
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
		a.MarkStale(context.Background(), ports, agent.DefaultStaleAfter)

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
	rootCmd.AddCommand(dockerCmd)
}

var listJSON bool
var showAll bool

// nonNil ensures a JSON list endpoint always marshals to "[]" for an
// empty result rather than "null" (Go's encoding/json for a nil slice),
// which a caller expecting an array to iterate would otherwise choke on.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func filterPorts(ports []models.Port) []models.Port {
	if showAll {
		return ports
	}
	var filtered []models.Port
	for _, p := range ports {
		// Hide processes we identify as system/background noise
		if p.Process != nil && platform.IsSystemOrBackgroundProcess(p.Process.Name) {
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

var (
	listRange      string
	listStale      bool
	listStaleAfter time.Duration
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List running processes and listening ports",
	Example: "  localpilot list\n" +
		"  localpilot list --all               # include system/background processes\n" +
		"  localpilot list --range 3000-4000   # only a known dev-server port band\n" +
		"  localpilot list --stale             # old, idle processes with no established connections\n" +
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

		a.MarkStale(context.Background(), ports, listStaleAfter)
		if listStale {
			ports = slices.DeleteFunc(ports, func(p models.Port) bool { return !p.Stale })
		}

		if listJSON {
			return printJSON(cmd.OutOrStdout(), nonNil(ports))
		}
		output.PrintList(ports)
		return nil
	},
}

var dockerJSON bool

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "List running Docker containers and their published host ports",
	Example: "  localpilot docker\n" +
		"  localpilot docker --json | jq '.[].name'",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		containers := platform.ListDockerContainers(context.Background())
		if dockerJSON {
			return printJSON(cmd.OutOrStdout(), nonNil(containers))
		}
		output.PrintDockerList(containers)
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

		project := platform.DetectProject(proc.Cwd)

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

// termOpts holds the flags shared by kill and free.
type termOpts struct {
	force, json, container bool
}

var killOpts, freeOpts termOpts

func (o *termOpts) register(c *cobra.Command) {
	c.Flags().BoolVar(&o.force, "force", false, "Skip confirmation and force kill")
	c.Flags().BoolVar(&o.json, "json", false, "Output as JSON (requires --force)")
	c.Flags().BoolVar(&o.container, "container", false, "With --force, stop the owning Docker container instead of killing the process")
}

func (o *termOpts) checkJSON() error {
	if o.json && !o.force {
		return fmt.Errorf("--json requires --force")
	}
	return nil
}

func (o *termOpts) checkInteractive(verb string) error {
	if !o.force && !isInteractiveStdin() {
		return fmt.Errorf("refusing to prompt for confirmation: stdin is not a terminal; pass --force to %s non-interactively", verb)
	}
	return nil
}

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
// containerFlag (--container) means the user has explicitly chosen to
// stop the container. Like every other termination path in kill/free,
// that's only carried out without confirmation when force is also true;
// otherwise the user is shown what's about to be stopped and asked to
// confirm, exactly as an ordinary kill would.
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
// just the one at hand. Interactively (and --container not given), the
// user is asked which action to take. In --force mode there's no one to
// ask, so this is never guessed: blocked is returned true unless
// --container was passed explicitly, and the caller must refuse to
// proceed rather than silently risk it.
func resolveContainerStop(container *models.Container, proc *models.Process, force, containerFlag, soleContainer bool) (stop, blocked bool) {
	if container == nil {
		return false, false
	}
	if containerFlag {
		if force {
			return true, false
		}
		output.PrintContainerStopConfirmation(container)
		return confirm(), false
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
func soleRunningContainer(ctx context.Context, c *models.Container) bool {
	if c == nil {
		return true
	}
	n, ok := platform.RunningContainerCount(ctx)
	return ok && n <= 1
}

// termTarget is what kill/free is about to act on: the owning process, the
// Docker container behind it (if any), the port it was resolved from (nil
// when targeted by PID), and how to describe it in a confirmation prompt.
type termTarget struct {
	proc      *models.Process
	container *models.Container
	port      *int
	confirm   func()
}

// terminate stops the target's container or kills its process, confirming
// first unless o.force, and reports the result as JSON or text.
func terminate(ctx context.Context, out io.Writer, o *termOpts, t termTarget) error {
	stop, blocked := resolveContainerStop(t.container, t.proc, o.force, o.container, soleRunningContainer(ctx, t.container))
	subject := fmt.Sprintf("process %d", t.proc.PID)
	where := ""
	if t.port != nil {
		subject = fmt.Sprintf("port %d", *t.port)
		where = fmt.Sprintf(" (port %d)", *t.port)
	}
	if blocked {
		return fmt.Errorf("%s is owned by Docker container %q, and other containers are also running: pass --container to stop it, since killing the process risks affecting them too", subject, t.container.Name)
	}

	res := terminationResult{Port: t.port, Terminated: true}
	var msg string
	if stop {
		if err := platform.StopContainer(ctx, t.container.ID); err != nil {
			return fmt.Errorf("failed to stop container %s: %w", t.container.Name, err)
		}
		res.Container, res.ContainerStopped = t.container.Name, true
		msg = fmt.Sprintf("Container %s%s stopped.", t.container.Name, where)
	} else {
		if !o.force {
			t.confirm()
			if !confirm() {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		if err := platform.KillPID(ctx, t.proc.PID, o.force); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", t.proc.PID, err)
		}
		res.PID = t.proc.PID
		msg = fmt.Sprintf("Process %d%s terminated.", t.proc.PID, where)
	}

	if o.json {
		return printJSON(out, res)
	}
	fmt.Println(msg)
	return nil
}

// findPortTarget resolves target as a port that is in use by an identified
// process. It returns a nil binding if target isn't a port number or the
// port isn't in use.
func findPortTarget(ctx context.Context, a *agent.Agent, target string) (*models.PortBinding, int, error) {
	port, err := parsePort(target)
	if err != nil {
		return nil, 0, nil
	}
	binding, err := a.FindPort(ctx, port)
	if err != nil {
		return nil, 0, err
	}
	if binding.InUse && binding.Process != nil {
		return binding, port, nil
	}
	return nil, 0, nil
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
		if err := killOpts.checkJSON(); err != nil {
			return err
		}

		a, err := agent.New()
		if err != nil {
			return err
		}

		if err := killOpts.checkInteractive("kill"); err != nil {
			return err
		}

		target := args[0]
		ctx := context.Background()

		// PID and port ranges overlap on Linux (pid_max is commonly well
		// under 65535), so a bare number like "3000" is ambiguous. Ports are
		// the documented, primary use of `kill <target>` (see README), and
		// resolving to the wrong PID here means killing an unrelated
		// process — so a port match always wins over a PID match.
		binding, port, err := findPortTarget(ctx, a, target)
		if err != nil {
			return err
		}
		if binding != nil {
			return terminate(ctx, cmd.OutOrStdout(), &killOpts, termTarget{
				proc: binding.Process, container: binding.Container, port: &port,
				confirm: func() { output.PrintKillConfirmation(binding) },
			})
		}

		pid, pidErr := parsePID(target)
		if pidErr != nil {
			return fmt.Errorf("invalid target: must be a PID or port number")
		}

		proc, err := a.InspectProcess(ctx, pid)
		if err != nil {
			return fmt.Errorf("no listening port and no process found for %q", target)
		}
		project := platform.DetectProject(proc.Cwd)

		return terminate(ctx, cmd.OutOrStdout(), &killOpts, termTarget{
			proc: proc, container: proc.Container,
			confirm: func() { output.PrintKillByPIDConfirmation(proc, project) },
		})
	},
}

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
		if err := freeOpts.checkJSON(); err != nil {
			return err
		}

		port, err := parsePort(args[0])
		if err != nil {
			return err
		}

		a, err := agent.New()
		if err != nil {
			return err
		}

		if err := freeOpts.checkInteractive("free"); err != nil {
			return err
		}

		ctx := context.Background()
		binding, err := a.FindPort(ctx, port)
		if err != nil {
			return err
		}

		if !binding.InUse || binding.Process == nil {
			if freeOpts.json {
				return printJSON(cmd.OutOrStdout(), terminationResult{Port: &port, Terminated: false})
			}
			fmt.Printf("Port %d is not in use.\n", port)
			return nil
		}

		return terminate(ctx, cmd.OutOrStdout(), &freeOpts, termTarget{
			proc: binding.Process, container: binding.Container, port: &port,
			confirm: func() { output.PrintKillConfirmation(binding) },
		})
	},
}

var watchInterval time.Duration

// resolveWatchTarget mirrors killCmd's PID/port disambiguation: a bare
// number is ambiguous between the two, and a port match always wins
// because `watch <target>` is documented primarily for ports (see README).
func resolveWatchTarget(ctx context.Context, a *agent.Agent, target string) (port int, pid int32, isPort bool, err error) {
	if binding, p, findErr := findPortTarget(ctx, a, target); findErr == nil && binding != nil {
		return p, 0, true, nil
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
	project := platform.DetectProject(proc.Cwd)
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
			return printJSON(cmd.OutOrStdout(), nonNil(conflicts))
		}
		output.PrintDoctor(conflicts)
		return nil
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all processes (including system/background)")
	listCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all processes (including system/background)")
	killOpts.register(killCmd)
	freeOpts.register(freeCmd)
	watchCmd.Flags().DurationVar(&watchInterval, "interval", time.Second, "Refresh interval (e.g. 1s, 500ms)")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	listCmd.Flags().BoolVar(&listStale, "stale", false, "Only show stale ports (old, idle, no established connections)")
	listCmd.Flags().DurationVar(&listStaleAfter, "stale-after", agent.DefaultStaleAfter, "Minimum process age before it can be considered stale")
	listCmd.Flags().StringVar(&listRange, "range", "", "Only show ports within <start>-<end>, e.g. 3000-4000")
	dockerCmd.Flags().BoolVar(&dockerJSON, "json", false, "Output as JSON")
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
