---
title: "Quickstart"
weight: 2
---

# Quickstart

```bash
# Show dashboard
localpilot

# Find what's using a port
localpilot port 3000

# Inspect a process
localpilot inspect 12345

# Kill by port or PID (with confirmation)
localpilot kill 3000
localpilot kill 12345 --force

# Free a port (always treats the argument as a port, never a PID)
localpilot free 3000

# List all listening ports
localpilot list
localpilot list --range 3000-4000

# Watch a port or process for changes (Ctrl+C to exit)
localpilot watch 3000
localpilot watch 12345 --interval 500ms
```

See the full [command reference](/commands/) for every flag.
