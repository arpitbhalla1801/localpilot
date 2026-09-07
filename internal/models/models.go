package models

import "time"

// Process represents a running OS process.
type Process struct {
	PID         int32
	Name        string
	Command     string
	Cwd         string
	ParentPID   int32
	ParentName  string
	CPUPercent  float64
	MemoryBytes uint64
	StartTime   time.Time
	Environment map[string]string
	OpenPorts   []int
}

// Port represents a network port binding.
type Port struct {
	Number   int
	Protocol string
	Address  string
	PID      int32
	Process  *Process
}

// Project represents a detected development project.
type Project struct {
	Name       string
	Path       string
	Repository string
	Branch     string
	Framework  string
}

// Service represents a logical service within a project.
type Service struct {
	Name    string
	Project *Project
	Process *Process
	Ports   []int
	Status  string
}

// PortBinding summarizes a port with its owning process and project context.
type PortBinding struct {
	Port    int
	Process *Process
	Project *Project
	InUse   bool
}
