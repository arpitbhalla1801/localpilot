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

// nameOrRestricted returns name, or "Restricted" when the OS withheld it.
func nameOrRestricted(name string) string {
	if name == "" {
		return "Restricted"
	}
	return name
}

func EnrichProcess(ctx context.Context, pid int32) (*models.Process, error) {
	proc, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("process %d not found: %w", pid, err)
	}

	// Fields the OS won't reveal (permissions, races) are left zero-valued.
	p := &models.Process{PID: pid}
	name, _ := proc.NameWithContext(ctx)
	p.Name = nameOrRestricted(name)
	p.Command, _ = proc.CmdlineWithContext(ctx)
	p.Cwd, _ = proc.CwdWithContext(ctx)
	p.CPUPercent, _ = proc.CPUPercentWithContext(ctx)
	if mem, _ := proc.MemoryInfoWithContext(ctx); mem != nil {
		p.MemoryBytes = mem.RSS
	}
	if created, _ := proc.CreateTimeWithContext(ctx); created > 0 {
		p.StartTime = time.UnixMilli(created)
	}
	if parent, _ := proc.ParentWithContext(ctx); parent != nil {
		p.ParentPID = parent.Pid
		parentName, _ := parent.NameWithContext(ctx)
		p.ParentName = nameOrRestricted(parentName)
	}
	env, _ := proc.EnvironWithContext(ctx)
	p.Environment = security.MaskEnvironment(parseEnviron(env))
	p.OpenPorts, _ = listPortsForPID(ctx, pid)
	return p, nil
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

// isBoundSocket reports whether a connection represents something
// actually reachable on the host: a TCP socket in LISTEN state, or a UDP
// socket bound to a port. UDP is connectionless, so "listening" isn't a
// meaningful concept for it — gopsutil (and most OSes) typically reports
// UDP sockets with an empty/"NONE" status even while actively bound and
// receiving, so filtering UDP on Status == "LISTEN" like TCP would
// silently exclude every UDP-bound port (DNS resolvers, QUIC/HTTP3 dev
// servers, etc.) from every port command.
func isBoundSocket(c net.ConnectionStat) bool {
	if c.Type == socketTypeUDP {
		return c.Laddr.Port > 0
	}
	return c.Status == "LISTEN"
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
		if isBoundSocket(c) && portNum > 0 && !seen[portNum] {
			seen[portNum] = true
			ports = append(ports, portNum)
		}
	}
	return ports, nil
}

func FindListeningPort(ctx context.Context, port int) (*models.PortBinding, error) {
	conns, err := net.ConnectionsWithContext(ctx, "all")
	if err != nil {
		return nil, err
	}

	for _, c := range conns {
		if !isBoundSocket(c) || int(c.Laddr.Port) != port {
			continue
		}

		if c.Pid == 0 {
			continue
		}

		proc, err := EnrichProcess(ctx, c.Pid)
		if err != nil {
			continue
		}

		project := DetectProject(proc.Cwd)

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

func ListAllListeningPorts(ctx context.Context) ([]models.Port, error) {
	conns, err := net.ConnectionsWithContext(ctx, "all")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var ports []models.Port

	for _, c := range conns {
		if !isBoundSocket(c) || c.Laddr.Port == 0 {
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
			proc, err := EnrichProcess(ctx, c.Pid)
			if err == nil {
				port.Process = proc
			}
		}

		ports = append(ports, port)
	}

	return ports, nil
}

func KillPID(ctx context.Context, pid int32, force bool) error {
	proc, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	if force {
		return proc.KillWithContext(ctx)
	}

	// Without --force, try a graceful terminate (SIGTERM) first, falling
	// back to a forced kill (SIGKILL) if that fails — including for
	// reasons unrelated to "the process ignored SIGTERM" (e.g. permission
	// denied, the process already exited). If the fallback also fails,
	// both errors are reported: the terminate error usually carries the
	// more useful root cause, and discarding it (as this used to do)
	// leaves only the less informative kill error to debug from.
	terminateErr := proc.TerminateWithContext(ctx)
	if terminateErr == nil {
		return nil
	}
	if killErr := proc.KillWithContext(ctx); killErr != nil {
		return fmt.Errorf("terminate failed: %v; force kill also failed: %w", terminateErr, killErr)
	}
	return nil
}

func DetectProject(cwd string) *models.Project {
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

// frameworkMarkers is checked in order, not as a map: Go map iteration is
// randomized per run, which previously made detectFramework's result
// nondeterministic for any project with more than one marker file present
// (e.g. a monorepo with both package.json and go.mod). docker-compose.yml
// is checked first since it's the strongest, least ambiguous signal —
// a project orchestrated via Compose is a meaningfully different context
// regardless of which languages happen to live inside it.
var frameworkMarkers = []struct {
	file      string
	framework string
}{
	{"docker-compose.yml", "Docker Compose"},
	{"package.json", "Node.js"},
	{"go.mod", "Go"},
	{"Cargo.toml", "Rust"},
	{"pyproject.toml", "Python"},
	{"requirements.txt", "Python"},
	{"pom.xml", "Java/Maven"},
	{"build.gradle", "Java/Gradle"},
}

func detectFramework(projectPath string) string {
	for _, marker := range frameworkMarkers {
		if _, err := os.Stat(filepath.Join(projectPath, marker.file)); err == nil {
			return marker.framework
		}
	}
	return ""
}

// Socket type constants as reported by gopsutil/net (SOCK_STREAM/SOCK_DGRAM).
const (
	socketTypeTCP = 1
	socketTypeUDP = 2
)

func socketTypeName(sockType uint32) string {
	switch sockType {
	case socketTypeTCP:
		return "tcp"
	case socketTypeUDP:
		return "udp"
	default:
		return fmt.Sprintf("socket-%d", sockType)
	}
}
