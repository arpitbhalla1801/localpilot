package platform

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// activitySampleWindow is how long IdlePIDs watches CPU/IO counters. Short
// on purpose: it's paid once per list, for the (few) old candidates only.
const activitySampleWindow = 250 * time.Millisecond

// activitySample is one reading of a process's cumulative CPU time and I/O
// bytes. A field group that couldn't be read (permissions, unsupported OS)
// is marked unread so it can't be mistaken for "no activity".
type activitySample struct {
	cpu             float64
	io              uint64
	cpuRead, ioRead bool
}

func readActivity(ctx context.Context, pid int32) activitySample {
	var s activitySample
	p, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return s
	}
	if t, err := p.TimesWithContext(ctx); err == nil {
		s.cpu, s.cpuRead = t.User+t.System, true
	}
	if io, err := p.IOCountersWithContext(ctx); err == nil {
		s.io, s.ioRead = io.ReadBytes+io.WriteBytes, true
	}
	return s
}

// idleBetween reports whether two samples show no activity. It is false
// when neither counter was readable both times: unknown is never idle.
func idleBetween(a, b activitySample) bool {
	idle, known := true, false
	if a.cpuRead && b.cpuRead {
		known = true
		idle = idle && b.cpu <= a.cpu
	}
	if a.ioRead && b.ioRead {
		known = true
		idle = idle && b.io <= a.io
	}
	return idle && known
}

// IdlePIDs returns which of pids show positive evidence of being idle: no
// established connection (inbound or outbound) and no CPU or I/O progress
// over a short sample window. A PID whose counters can't be read (e.g. a
// process owned by another user on Windows) is left out, i.e. treated as
// active/unknown, never as idle.
func IdlePIDs(ctx context.Context, pids []int32) map[int32]bool {
	idle := map[int32]bool{}
	if len(pids) == 0 {
		return idle
	}

	busy := map[int32]bool{}
	if conns, err := net.ConnectionsWithContext(ctx, "tcp"); err == nil {
		for _, c := range conns {
			if c.Status == "ESTABLISHED" {
				busy[c.Pid] = true
			}
		}
	}

	before := map[int32]activitySample{}
	for _, pid := range pids {
		if !busy[pid] {
			before[pid] = readActivity(ctx, pid)
		}
	}
	if len(before) == 0 {
		return idle
	}

	select {
	case <-time.After(activitySampleWindow):
	case <-ctx.Done():
		return idle
	}
	for pid, b := range before {
		if idleBetween(b, readActivity(ctx, pid)) {
			idle[pid] = true
		}
	}
	return idle
}
