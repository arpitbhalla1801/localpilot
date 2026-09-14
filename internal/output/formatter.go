package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/localpilot/localpilot/internal/models"
)

func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
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

func formatBytes(bytes uint64) string {
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

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format("15:04:05")
}

func formatStarted(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	ago := time.Since(t)
	if ago < 24*time.Hour {
		return formatDuration(ago) + " ago"
	}
	return t.Format("Jan 2, 15:04")
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
	fmt.Println()

	proc := binding.Process
	fmt.Println("Process")
	fmt.Println("────────────────────────────────────")
	fmt.Printf("PID             %d\n", proc.PID)
	fmt.Printf("Name            %s\n", sanitize(proc.Name))
	fmt.Printf("CPU             %.1f%%\n", proc.CPUPercent)
	fmt.Printf("Memory          %s\n", formatBytes(proc.MemoryBytes))
	fmt.Printf("Started         %s\n", formatTime(proc.StartTime))

	fmt.Println()
	fmt.Println("Command")
	fmt.Println("────────────────────────────────────")
	if proc.Command != "" {
		fmt.Println(sanitize(proc.Command))
	} else {
		fmt.Println("(unknown)")
	}

	if proc.Cwd != "" {
		fmt.Println()
		fmt.Println("Working Directory")
		fmt.Println("────────────────────────────────────")
		fmt.Println(sanitize(shortenPath(proc.Cwd)))
	}

	if binding.Project != nil {
		fmt.Println()
		fmt.Println("Git Repository")
		fmt.Println("────────────────────────────────────")
		fmt.Println(sanitize(binding.Project.Name))
		if binding.Project.Branch != "" {
			fmt.Printf("Branch: %s\n", sanitize(binding.Project.Branch))
		}
		if binding.Project.Framework != "" {
			fmt.Printf("Framework: %s\n", sanitize(binding.Project.Framework))
		}
	}

	if proc.ParentName != "" {
		fmt.Println()
		fmt.Println("Parent Process")
		fmt.Println("────────────────────────────────────")
		fmt.Printf("%s (PID %d)\n", sanitize(proc.ParentName), proc.ParentPID)
	}

	fmt.Println("────────────────────────────────────")
	fmt.Println("Suggested actions:")
	fmt.Printf("  localpilot inspect %d\n", proc.PID)
	fmt.Printf("  localpilot kill %d\n", proc.PID)
	if binding.Project != nil {
		fmt.Printf("  Project: %s\n", sanitize(shortenPath(binding.Project.Path)))
	}
}

// PrintInspect displays detailed process information.
func PrintInspect(proc *models.Process, project *models.Project) {
	fmt.Println("PROCESS")
	fmt.Println("────────────────────────────────────")
	fmt.Printf("PID             %d\n", proc.PID)
	fmt.Printf("Name            %s\n", sanitize(proc.Name))
	fmt.Printf("CPU             %.1f%%\n", proc.CPUPercent)
	fmt.Printf("Memory          %s\n", formatBytes(proc.MemoryBytes))
	fmt.Printf("Started         %s\n", formatTime(proc.StartTime))

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
	fmt.Printf("%-20s %-12s %-8s %-8s %s\n", "ADDRESS", "PROCESS", "PORT", "PID", "STATUS")
	fmt.Println(strings.Repeat("─", 60))

	for _, p := range ports {
		name := "-"
		pid := "-"
		status := "● RUNNING"

		if p.Process != nil {
			name = sanitize(p.Process.Name)
			pid = fmt.Sprintf("%d", p.Process.PID)

			if !p.Process.StartTime.IsZero() && time.Since(p.Process.StartTime) > 48*time.Hour && !IsSystemOrBackgroundProcess(name) {
				status = "⚠ STALE"
			}
		} else if p.PID > 0 {
			pid = fmt.Sprintf("%d", p.PID)
		}

		addr := fmt.Sprintf("%s:%d", p.Address, p.Number)
		fmt.Printf("%-20s %-12s %-8s %-8s %s\n", truncate(addr, 20), truncate(name, 12), fmt.Sprintf("%d", p.Number), pid, status)
	}
}

// PrintKillConfirmation shows what will be killed.
func PrintKillConfirmation(binding *models.PortBinding) {
	if binding.Process != nil {
		fmt.Printf("Port %d is used by:\n", binding.Port)
		fmt.Printf("  %s\n", sanitize(binding.Process.Name))
		fmt.Printf("  PID: %d\n", binding.Process.PID)
		if binding.Project != nil {
			fmt.Printf("  Project: %s\n", sanitize(binding.Project.Name))
		}
	} else {
		fmt.Printf("Process PID %d\n", binding.Process.PID)
	}
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

// PrintDashboard is the default view when running `localpilot`.
func PrintDashboard(ports []models.Port) {
	fmt.Println()
	fmt.Println("┌────────────────────────────────────────────────────────────────────────┐")
	fmt.Println("│                          LOCALPILOT                                  │")
	fmt.Println("├──────────┬──────────┬─────────────────────┬──────────┬─────────────────┤")
	fmt.Println("│ PROJECT  │ SERVICE  │ PORT                │ PROCESS  │ STATUS          │")
	fmt.Println("├──────────┼──────────┼─────────────────────┼──────────┼─────────────────┤")

	if len(ports) == 0 {
		fmt.Println("│ (no listening ports detected)                                        │")
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
						project = truncate(sanitize(detected), 10)
					}
					service = truncate(sanitize(filepath.Base(p.Process.Cwd)), 10)
				}
				if !p.Process.StartTime.IsZero() && time.Since(p.Process.StartTime) > 48*time.Hour && !IsSystemOrBackgroundProcess(processName) {
					status = "⚠ Stale"
				}
			}

			portStr := fmt.Sprintf("%s:%d", p.Address, p.Number)
			fmt.Printf("│ %-8s │ %-8s │ %-19s │ %-8s │ %-15s │\n",
				truncate(project, 8),
				truncate(service, 8),
				portStr,
				truncate(processName, 8),
				truncate(status, 15),
			)
		}
	}

	fmt.Println("└──────────┴──────────┴─────────────────────┴──────────┴─────────────────┘")
	fmt.Println()
	fmt.Printf("  %d ports listening\n", len(ports))
	fmt.Println()
	fmt.Println("Commands: localpilot port <PORT> | inspect <PID> | kill <PID|PORT> | list")
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func detectProjectName(cwd string) string {
	home, _ := os.UserHomeDir()
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
	if strings.HasPrefix(cwd, home) {
		return filepath.Base(cwd)
	}
	return filepath.Base(cwd)
}

func IsSystemOrBackgroundProcess(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	// Core Windows System processes
	case "system", "svchost.exe", "lsass.exe", "wininit.exe", "services.exe", "spoolsv.exe", "csrss.exe", "smss.exe", "explorer.exe", "cmrcservice.exe", "pangps.exe", "jhi_service.exe", "wepsvc.exe", "fppsvc.exe", "searchindexer.exe":
		return true
	// Background services / daemons (Windows, macOS, Linux)
	case "mysqld.exe", "postgres.exe", "docker.exe", "wslrelay.exe", "code.exe",
		"mysqld", "postgres", "dockerd", "docker", "containerd",
		"controlcenter", "rapportd", "coreaudiod", "corebrightnessd",
		"coreservicesd", "cfprefsd", "distnoted", "usernoted", "notifyd",
		"bluetoothd", "wifianalyticsd", "wifivelocityd", "locationd",
		"launchd", "systemd", "systemd-resolved", "systemd-journald",
		"systemd-logind", "systemd-udevd", "dbus-daemon", "cron", "crond",
		"code helper", "code helper (plugin)", "code helper (renderer)", "code helper (gpu)":
		return true
	}
	return false
}
