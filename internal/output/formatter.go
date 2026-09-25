package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if rest, ok := strings.CutPrefix(path, home); ok {
		return "~" + rest
	}
	return path
}

// sanitize neutralizes control characters (including ANSI/CSI/OSC escape
// sequences, embedded newlines, and other C0 control bytes) in strings
// that originate from process metadata — cwd, command, name, project
// info — before they're written to the terminal. That data comes from
// whatever process the user asks to inspect, so it must never be trusted
// to behave like a normal, single-line value.
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			fmt.Fprintf(&b, "\\x%02x", r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// FormatBytes renders a byte count in human units.
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}

// FormatTime renders a clock time, or "unknown" for the zero time.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format("15:04:05")
}

const rule = "────────────────────────────────────"

// section prints a blank line, a section title and a rule under it.
func section(title string) {
	fmt.Println()
	fmt.Println(title)
	fmt.Println(rule)
}

// PrintPortDoctor displays port occupancy details.
func PrintPortDoctor(binding *models.PortBinding) {
	fmt.Println("PORT DOCTOR")
	fmt.Printf("Port: %d\n", binding.Port)

	if !binding.InUse {
		fmt.Println("Status: FREE")
		fmt.Println()
		fmt.Println("Port is available.")
		return
	}

	fmt.Println("Status: IN USE")

	proc := binding.Process
	section("Process")
	fmt.Printf("PID             %d\n", proc.PID)
	fmt.Printf("Name            %s\n", sanitize(proc.Name))
	fmt.Printf("CPU             %.1f%%\n", proc.CPUPercent)
	fmt.Printf("Memory          %s\n", FormatBytes(proc.MemoryBytes))
	fmt.Printf("Started         %s\n", FormatTime(proc.StartTime))

	section("Command")
	if proc.Command != "" {
		fmt.Println(sanitize(proc.Command))
	} else {
		fmt.Println("(unknown)")
	}

	if proc.Cwd != "" {
		section("Working Directory")
		fmt.Println(sanitize(shortenPath(proc.Cwd)))
	}

	if binding.Container != nil {
		section("Docker Container")
		fmt.Printf("Name            %s\n", sanitize(binding.Container.Name))
		fmt.Printf("Image           %s\n", sanitize(binding.Container.Image))
		fmt.Printf("ID              %s\n", sanitize(binding.Container.ID))
	}

	if binding.WSL != nil {
		section("WSL Process")
		fmt.Printf("Distro          %s\n", sanitize(binding.WSL.Distro))
		fmt.Printf("PID             %d\n", binding.WSL.PID)
		fmt.Printf("Name            %s\n", sanitize(binding.WSL.Name))
	}

	if binding.Project != nil {
		section("Git Repository")
		fmt.Println(sanitize(binding.Project.Name))
		if binding.Project.Branch != "" {
			fmt.Printf("Branch: %s\n", sanitize(binding.Project.Branch))
		}
		if binding.Project.Framework != "" {
			fmt.Printf("Framework: %s\n", sanitize(binding.Project.Framework))
		}
	}

	if proc.ParentName != "" {
		section("Parent Process")
		fmt.Printf("%s (PID %d)\n", sanitize(proc.ParentName), proc.ParentPID)
	}

	fmt.Println(rule)
	fmt.Println("Suggested actions:")
	fmt.Printf("  localpilot inspect %d\n", proc.PID)
	fmt.Printf("  localpilot kill %d\n", proc.PID)
	if binding.Project != nil {
		fmt.Printf("  Project: %s\n", sanitize(shortenPath(binding.Project.Path)))
	}
}

// nameAndPID returns display forms of a port's owning process name and PID,
// or "-" when unknown.
func nameAndPID(p models.Port) (name, pid string) {
	name, pid = "-", "-"
	if p.Process != nil {
		name = sanitize(p.Process.Name)
		pid = fmt.Sprintf("%d", p.Process.PID)
	} else if p.PID > 0 {
		pid = fmt.Sprintf("%d", p.PID)
	}
	return name, pid
}

// PrintInspect displays detailed process information.
func PrintInspect(proc *models.Process, project *models.Project) {
	fmt.Println("PROCESS")
	fmt.Println("────────────────────────────────────")
	fmt.Printf("PID             %d\n", proc.PID)
	fmt.Printf("Name            %s\n", sanitize(proc.Name))
	fmt.Printf("CPU             %.1f%%\n", proc.CPUPercent)
	fmt.Printf("Memory          %s\n", FormatBytes(proc.MemoryBytes))
	fmt.Printf("Started         %s\n", FormatTime(proc.StartTime))

	if proc.Container != nil {
		fmt.Println()
		fmt.Println("Docker Container")
		fmt.Printf("  %s (%s)\n", sanitize(proc.Container.Name), sanitize(proc.Container.Image))
	}

	if proc.WSL != nil {
		fmt.Println()
		fmt.Println("WSL Process")
		fmt.Printf("  %s (PID %d in %s)\n", sanitize(proc.WSL.Name), proc.WSL.PID, sanitize(proc.WSL.Distro))
	}

	if !proc.StartTime.IsZero() {
		fmt.Printf("Runtime         %s\n", formatDuration(time.Since(proc.StartTime)))
	}

	fmt.Println()
	fmt.Println("Command")
	if proc.Command != "" {
		fmt.Printf("  %s\n", sanitize(proc.Command))
	} else {
		fmt.Println("  (unknown)")
	}

	if proc.Cwd != "" {
		fmt.Println()
		fmt.Println("Working Directory")
		fmt.Printf("  %s\n", sanitize(shortenPath(proc.Cwd)))
	}

	if project != nil {
		fmt.Println()
		fmt.Println("Project")
		fmt.Printf("  %s\n", sanitize(project.Name))
		if project.Branch != "" {
			fmt.Printf("Git Branch\n  %s\n", sanitize(project.Branch))
		}
		if project.Framework != "" {
			fmt.Printf("Framework\n  %s\n", sanitize(project.Framework))
		}
	}

	if proc.ParentName != "" {
		fmt.Println()
		fmt.Println("Parent")
		fmt.Printf("  %s (PID %d)\n", sanitize(proc.ParentName), proc.ParentPID)
	}

	if len(proc.OpenPorts) > 0 {
		fmt.Println()
		fmt.Println("Open Ports")
		for _, p := range proc.OpenPorts {
			fmt.Printf("  %d\n", p)
		}
	}

	if len(proc.Environment) > 0 {
		fmt.Println()
		fmt.Println("Environment")
		count := 0
		for key, val := range proc.Environment {
			if count >= 20 {
				fmt.Printf("  ... and %d more\n", len(proc.Environment)-20)
				break
			}
			fmt.Printf("  %s=%s\n", sanitize(key), sanitize(val))
			count++
		}
	}
}

// PrintList displays a table of listening ports and processes.
func PrintList(ports []models.Port) {
	if len(ports) == 0 {
		fmt.Println("No listening ports found.")
		return
	}

	fmt.Println("LOCALPILOT")
	fmt.Printf("%-20s %-12s %-8s %-8s %-10s %s\n", "ADDRESS", "PROCESS", "PORT", "PID", "CONTAINER", "STATUS")
	fmt.Println(strings.Repeat("─", 72))

	for _, p := range ports {
		name, pid := nameAndPID(p)
		status := "● RUNNING"
		container := "-"

		if p.Stale {
			status = "⚠ STALE"
		}

		if p.Container != nil {
			container = sanitize(p.Container.Name)
		} else if p.WSL != nil {
			container = "wsl:" + sanitize(p.WSL.Distro)
		}

		addr := fmt.Sprintf("%s:%d", p.Address, p.Number)
		fmt.Printf("%-20s %-12s %-8s %-8s %-10s %s\n", Truncate(addr, 20), Truncate(name, 12), fmt.Sprintf("%d", p.Number), pid, Truncate(container, 10), status)
	}
}

// PrintDockerList renders running containers and the host ports they publish.
func PrintDockerList(containers []*models.Container) {
	if len(containers) == 0 {
		fmt.Println("No running containers with published ports found.")
		return
	}

	fmt.Println("DOCKER")
	fmt.Printf("%-24s %-28s %s\n", "CONTAINER", "IMAGE", "HOST PORTS")
	fmt.Println(strings.Repeat("─", 72))
	for _, c := range containers {
		ports := make([]string, len(c.Ports))
		for i, p := range c.Ports {
			ports[i] = fmt.Sprintf("%d", p)
		}
		fmt.Printf("%-24s %-28s %s\n", Truncate(sanitize(c.Name), 24), Truncate(sanitize(c.Image), 28), strings.Join(ports, ", "))
	}
}

// PrintKillConfirmation shows what will be killed.
func PrintKillConfirmation(binding *models.PortBinding) {
	if binding.Process == nil {
		fmt.Printf("Port %d is in use, but no owning process could be identified.\n", binding.Port)
		return
	}
	fmt.Printf("Port %d is used by:\n", binding.Port)
	fmt.Printf("  %s\n", sanitize(binding.Process.Name))
	fmt.Printf("  PID: %d\n", binding.Process.PID)
	if binding.Project != nil {
		fmt.Printf("  Project: %s\n", sanitize(binding.Project.Name))
	}
}

// PrintContainerConflict explains that a port is owned by a Docker
// container via an intermediary process, ahead of asking the user which
// one to stop.
func PrintContainerConflict(container *models.Container, proc *models.Process) {
	fmt.Printf("This port is published by Docker container %q (%s)", sanitize(container.Name), sanitize(container.Image))
	if proc != nil {
		fmt.Printf(", via process %s (PID %d)", sanitize(proc.Name), proc.PID)
	}
	fmt.Println(".")
	fmt.Println("Killing that process alone typically won't free the port — Docker keeps it bound to the container.")
}

// PrintContainerStopConfirmation shows what will be stopped when the user
// has explicitly requested stopping a container (--container) rather than
// killing the underlying process.
func PrintContainerStopConfirmation(container *models.Container) {
	fmt.Printf("About to stop Docker container %q (%s).\n", sanitize(container.Name), sanitize(container.Image))
}

// PrintKillByPIDConfirmation shows PID kill confirmation.
func PrintKillByPIDConfirmation(proc *models.Process, project *models.Project) {
	fmt.Printf("About to terminate:\n")
	fmt.Printf("  %s (PID %d)\n", sanitize(proc.Name), proc.PID)
	if proc.Command != "" {
		fmt.Printf("  Command: %s\n", sanitize(proc.Command))
	}
	if project != nil {
		fmt.Printf("  Project: %s\n", sanitize(project.Name))
	}
}

// PrintDoctor displays the results of a port-conflict scan.
func PrintDoctor(conflicts []models.Conflict) {
	fmt.Println("LOCALPILOT DOCTOR")
	fmt.Println("────────────────────────────────────")

	if len(conflicts) == 0 {
		fmt.Println("No port conflicts found.")
		return
	}

	for i, c := range conflicts {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("⚠ Port %d: %s\n", c.Port, sanitize(c.Message))
		for _, p := range c.Ports {
			name, pid := nameAndPID(p)
			fmt.Printf("  %s:%d  %-12s  PID %s\n", p.Address, p.Number, name, pid)
		}
	}
}

// PrintDashboard is the default view when running `localpilot`.
func PrintDashboard(ports []models.Port) {
	fmt.Println()
	fmt.Println("┌────────────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                                           LOCALPILOT                                           │")
	fmt.Println("├──────────────────┬──────────────────┬─────────────────────┬──────────────────┬─────────────────┤")
	fmt.Println("│ PROJECT          │ SERVICE          │ PORT                │ PROCESS          │ STATUS          │")
	fmt.Println("├──────────────────┼──────────────────┼─────────────────────┼──────────────────┼─────────────────┤")

	if len(ports) == 0 {
		fmt.Println("│ (no listening ports detected)                                                                 │")
	} else {
		for _, p := range ports {
			project := "-"
			service := "-"
			processName := "-"
			status := "● Running"

			if p.Process != nil {
				processName = sanitize(p.Process.Name)
				if p.Process.Cwd != "" {
					detected := detectProjectName(p.Process.Cwd)
					if detected != "" {
						project = Truncate(sanitize(detected), 16)
					}
					service = Truncate(sanitize(filepath.Base(p.Process.Cwd)), 16)
				}
				if p.Stale {
					status = "⚠ Stale"
				}
			}
			if p.Container != nil {
				// The local process behind a container-published port is
				// typically docker-proxy or Docker Desktop's shared
				// backend — neither tells the user anything about which
				// container is actually involved, so show the container
				// name instead (same resolution #24 added to
				// port/inspect/list; the dashboard was the one place
				// still showing the raw process name).
				processName = sanitize(p.Container.Name)
			} else if p.WSL != nil {
				// Same idea for a port published from inside WSL: Windows
				// only sees wslrelay.exe, shared across every distro.
				processName = sanitize(p.WSL.Name)
			}

			portStr := fmt.Sprintf("%s:%d", p.Address, p.Number)
			fmt.Printf("│ %-16s │ %-16s │ %-19s │ %-16s │ %-15s │\n",
				Truncate(project, 16),
				Truncate(service, 16),
				portStr,
				Truncate(processName, 16),
				Truncate(status, 15),
			)
		}
	}

	fmt.Println("└──────────────────┴──────────────────┴─────────────────────┴──────────────────┴─────────────────┘")
	fmt.Println()
	fmt.Printf("  %d ports listening\n", len(ports))
	fmt.Println()
	fmt.Println("Commands: localpilot port <PORT> | inspect <PID> | kill <PID|PORT> | list")
}

// Truncate shortens s to max runes, ending in an ellipsis when cut.
func Truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func detectProjectName(cwd string) string {
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return filepath.Base(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Base(cwd)
}
