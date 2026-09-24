package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arpitbhalla1801/localpilot/internal/models"
)

// Conflict kinds reported by DetectConflicts.
const (
	// ConflictMultipleListeners means more than one distinct process is
	// currently bound to the same port number (e.g. one on 0.0.0.0,
	// another on 127.0.0.1, or via SO_REUSEPORT).
	ConflictMultipleListeners = "multiple_listeners"
	// ConflictSameDefaultPort means two or more different projects are
	// each configured (via .env or package.json) for the same port, even
	// though only one of them currently holds it.
	ConflictSameDefaultPort = "same_default_port"
)

// DetectConflicts scans the current listening ports (and, best-effort,
// their owning projects' config files) for port-related misconfigurations.
// This is intentionally scoped to port conflicts only (see issue #26):
// no general system health checks, no CPU/memory diagnostics.
func DetectConflicts(ports []models.Port) []models.Conflict {
	var conflicts []models.Conflict
	conflicts = append(conflicts, detectMultipleListeners(ports)...)
	conflicts = append(conflicts, detectSameDefaultPort(ports)...)
	return conflicts
}

// detectMultipleListeners flags ports that more than one distinct process
// is currently bound to. A single port normally has one owner, but the OS
// allows this via different bind addresses (0.0.0.0 vs 127.0.0.1) or
// SO_REUSEPORT, and it's a common source of "why is my request going to
// the wrong service" confusion.
func detectMultipleListeners(ports []models.Port) []models.Conflict {
	byPort := map[int][]models.Port{}
	for _, p := range ports {
		byPort[p.Number] = append(byPort[p.Number], p)
	}

	var conflicts []models.Conflict
	for port, group := range byPort {
		pids := map[int32]bool{}
		for _, p := range group {
			if p.Process != nil {
				pids[p.Process.PID] = true
			} else if p.PID > 0 {
				pids[p.PID] = true
			}
		}
		if len(pids) > 1 {
			conflicts = append(conflicts, models.Conflict{
				Kind:    ConflictMultipleListeners,
				Port:    port,
				Ports:   group,
				Message: fmt.Sprintf("%d different processes are bound to port %d", len(pids), port),
			})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].Port < conflicts[j].Port })
	return conflicts
}

// envPortRe matches a PORT=<number> assignment in a .env file.
var envPortRe = regexp.MustCompile(`(?m)^\s*PORT\s*=\s*"?'?(\d{2,5})"?'?\s*$`)

// scriptPortRe matches an explicit --port <number> (or --port=<number>) in
// a package.json's scripts section, the most common way dev servers pin a
// non-default port.
var scriptPortRe = regexp.MustCompile(`--port[= ]"?(\d{2,5})"?`)

// detectSameDefaultPort flags distinct projects whose own config (.env or
// package.json) declares the same port. Only one can actually be bound to
// it at a time, so seeing two configured for the same port is the classic
// "works on my machine until I run both" misconfiguration.
func detectSameDefaultPort(ports []models.Port) []models.Conflict {
	type entry struct {
		cwd  string
		port models.Port
	}
	declared := map[int][]entry{}
	seenCwd := map[string]bool{}

	for _, p := range ports {
		if p.Process == nil || p.Process.Cwd == "" {
			continue
		}
		cwd := p.Process.Cwd
		if seenCwd[cwd] {
			continue
		}
		seenCwd[cwd] = true

		dp, ok := declaredPort(cwd)
		if !ok {
			continue
		}
		declared[dp] = append(declared[dp], entry{cwd: cwd, port: p})
	}

	var conflicts []models.Conflict
	for port, entries := range declared {
		if len(entries) < 2 {
			continue
		}
		var group []models.Port
		var dirs []string
		for _, e := range entries {
			group = append(group, e.port)
			dirs = append(dirs, filepath.Base(e.cwd))
		}
		conflicts = append(conflicts, models.Conflict{
			Kind:    ConflictSameDefaultPort,
			Port:    port,
			Ports:   group,
			Message: fmt.Sprintf("%d projects are configured for port %d (%s)", len(entries), port, strings.Join(dirs, ", ")),
		})
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].Port < conflicts[j].Port })
	return conflicts
}

// declaredPort returns the port a project directory declares for itself,
// checked in .env (PORT=) then package.json (a --port flag in scripts).
// It's best-effort: any read/parse failure just means "not declared" here.
func declaredPort(cwd string) (int, bool) {
	if p, ok := declaredPortFrom(cwd, ".env", envPortRe); ok {
		return p, true
	}
	return declaredPortFrom(cwd, "package.json", scriptPortRe)
}

// declaredPortFrom returns the first port re captures from cwd/file, if valid.
func declaredPortFrom(cwd, file string, re *regexp.Regexp) (int, bool) {
	data, err := os.ReadFile(filepath.Join(cwd, file))
	if err != nil {
		return 0, false
	}
	m := re.FindSubmatch(data)
	if m == nil {
		return 0, false
	}
	port, err := strconv.Atoi(string(m[1]))
	if err != nil || port < 1 || port > 65535 {
		return 0, false
	}
	return port, true
}
