package models

import "time"

// Process represents a running OS process.
type Process struct {
	PID         int32             `json:"pid"`
	Name        string            `json:"name"`
	Command     string            `json:"command,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	ParentPID   int32             `json:"parentPid,omitempty"`
	ParentName  string            `json:"parentName,omitempty"`
	CPUPercent  float64           `json:"cpuPercent"`
	MemoryBytes uint64            `json:"memoryBytes"`
	StartTime   time.Time         `json:"startTime,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	OpenPorts   []int             `json:"openPorts,omitempty"`
	Container   *Container        `json:"container,omitempty"`
}

// Port represents a network port binding.
type Port struct {
	Number    int        `json:"number"`
	Protocol  string     `json:"protocol"`
	Address   string     `json:"address"`
	PID       int32      `json:"pid,omitempty"`
	Process   *Process   `json:"process,omitempty"`
	Container *Container `json:"container,omitempty"`
}

// Container identifies the Docker container publishing a port, resolved
// from `docker ps` when the port's owning process is docker-proxy (Linux)
// or a platform equivalent (e.g. Docker Desktop's VM-based proxying on
// macOS/Windows).
type Container struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

// Project represents a detected development project.
type Project struct {
	Name       string `json:"name"`
	Path       string `json:"path,omitempty"`
	Repository string `json:"repository,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Framework  string `json:"framework,omitempty"`
}

// Service represents a logical service within a project.
type Service struct {
	Name    string   `json:"name"`
	Project *Project `json:"project,omitempty"`
	Process *Process `json:"process,omitempty"`
	Ports   []int    `json:"ports,omitempty"`
	Status  string   `json:"status,omitempty"`
}

// PortBinding summarizes a port with its owning process and project context.
type PortBinding struct {
	Port      int        `json:"port"`
	Process   *Process   `json:"process,omitempty"`
	Project   *Project   `json:"project,omitempty"`
	Container *Container `json:"container,omitempty"`
	InUse     bool       `json:"inUse"`
}

// Conflict describes a single detected port conflict or misconfiguration,
// as produced by `localpilot doctor`.
type Conflict struct {
	Kind    string `json:"kind"`
	Port    int    `json:"port"`
	Message string `json:"message"`
	Ports   []Port `json:"ports,omitempty"`
}
