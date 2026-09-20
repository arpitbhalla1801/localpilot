package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/arpitbhalla1801/localpilot/internal/models"
	"github.com/arpitbhalla1801/localpilot/internal/security"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

func enrichProcess(ctx context.Context, pid int32) (*models.Process, error) {
	proc, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("process %d not found: %w", pid, err)
	}

	name, _ := proc.NameWithContext(ctx)
	if name == "" {
		name = "Restricted"
	}
	cmdline, _ := proc.CmdlineWithContext(ctx)
	cwd, _ := proc.CwdWithContext(ctx)
	cpu, _ := proc.CPUPercentWithContext(ctx)
	memInfo, _ := proc.MemoryInfoWithContext(ctx)
	createTime, _ := proc.CreateTimeWithContext(ctx)
	parent, _ := proc.ParentWithContext(ctx)

	var parentPID int32
	var parentName string
	if parent != nil {
		parentPID = parent.Pid
		parentName, _ = parent.NameWithContext(ctx)
		if parentName == "" {
			parentName = "Restricted"
		}
	}

	var memory uint64
	if memInfo != nil {
		memory = memInfo.RSS
	}

	var startTime time.Time
	if createTime > 0 {
		startTime = time.UnixMilli(createTime)
	}

	env, _ := proc.EnvironWithContext(ctx)
	envMap := parseEnviron(env)

	openPorts, _ := listPortsForPID(ctx, pid)

	return &models.Process{
		PID:         pid,
		Name:        name,
		Command:     cmdline,
		Cwd:         cwd,
		ParentPID:   parentPID,
		ParentName:  parentName,
		CPUPercent:  cpu,
		MemoryBytes: memory,
		StartTime:   startTime,
		Environment: security.MaskEnvironment(envMap),
		OpenPorts:   openPorts,
	}, nil
}

func parseEnviron(env []string) map[string]string {
	result := make(map[string]string, len(env))
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func listPortsForPID(ctx context.Context, pid int32) ([]int, error) {
	conns, err := net.ConnectionsPidWithContext(ctx, "all", pid)
	if err != nil {
		return nil, err
	}

	seen := make(map[int]bool)
	var ports []int
	for _, c := range conns {
		portNum := int(c.Laddr.Port)
		if c.Status == "LISTEN" && portNum > 0 && !seen[portNum] {
			seen[portNum] = true
			ports = append(ports, portNum)
		}
	}
	return ports, nil
}

func findListeningPort(ctx context.Context, port int) (*models.PortBinding, error) {
	conns, err := net.ConnectionsWithContext(ctx, "all")
	if err != nil {
		return nil, err
	}

	for _, c := range conns {
		if c.Status != "LISTEN" || int(c.Laddr.Port) != port {
			continue
		}

		if c.Pid == 0 {
			continue
		}

		proc, err := enrichProcess(ctx, c.Pid)
		if err != nil {
			continue
		}

		project := detectProject(proc.Cwd)

		return &models.PortBinding{
			Port:    port,
			Process: proc,
			Project: project,
			InUse:   true,
		}, nil
	}

	return &models.PortBinding{
		Port:  port,
		InUse: false,
	}, nil
}

func listAllListeningPorts(ctx context.Context) ([]models.Port, error) {
	conns, err := net.ConnectionsWithContext(ctx, "all")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var ports []models.Port

	for _, c := range conns {
		if c.Status != "LISTEN" || c.Laddr.Port == 0 {
			continue
		}

		key := fmt.Sprintf("%s:%d", c.Laddr.IP, c.Laddr.Port)
		if seen[key] {
			continue
		}
		seen[key] = true

		addr := c.Laddr.IP
		if addr == "" || addr == "0.0.0.0" || addr == "::" {
			addr = "localhost"
		}

		port := models.Port{
			Number:   int(c.Laddr.Port),
			Protocol: socketTypeName(c.Type),
			Address:  addr,
			PID:      c.Pid,
		}

		if c.Pid > 0 {
			proc, err := enrichProcess(ctx, c.Pid)
			if err == nil {
				port.Process = proc
			}
		}

		ports = append(ports, port)
	}

	return ports, nil
}

func killPID(ctx context.Context, pid int32, force bool) error {
	proc, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	if force {
		return proc.KillWithContext(ctx)
	}

	if err := proc.TerminateWithContext(ctx); err != nil {
		return proc.KillWithContext(ctx)
	}
	return nil
}

func detectProject(cwd string) *models.Project {
	if cwd == "" {
		return nil
	}

	gitRoot := findGitRoot(cwd)
	if gitRoot == "" {
		return &models.Project{
			Name: filepath.Base(cwd),
			Path: cwd,
		}
	}

	name := filepath.Base(gitRoot)
	branch := gitBranch(gitRoot)
	framework := detectFramework(gitRoot)

	return &models.Project{
		Name:       name,
		Path:       gitRoot,
		Repository: name,
		Branch:     branch,
		Framework:  framework,
	}
}

func findGitRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func gitBranch(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectFramework(projectPath string) string {
	markers := map[string]string{
		"package.json":       "Node.js",
		"pom.xml":            "Java/Maven",
		"build.gradle":       "Java/Gradle",
		"go.mod":             "Go",
		"pyproject.toml":     "Python",
		"requirements.txt":   "Python",
		"Cargo.toml":         "Rust",
		"docker-compose.yml": "Docker Compose",
	}

	for file, framework := range markers {
		if _, err := os.Stat(filepath.Join(projectPath, file)); err == nil {
			return framework
		}
	}
	return ""
}

func socketTypeName(sockType uint32) string {
	switch sockType {
	case 1:
		return "tcp"
	case 2:
		return "udp"
	default:
		return fmt.Sprintf("socket-%d", sockType)
	}
}
