# LocalPilot

Your command center for everything running on your machine.

LocalPilot is a developer-first CLI that helps you discover, understand, diagnose, and control everything running on localhost.

No more jumping between `lsof`, `ps`, `netstat`, `docker ps`, and `kill -9`.

## Quick Start

```bash
# Build
go build -o localpilot ./cmd/localpilot

# Show dashboard
./localpilot

# Find what's using a port
./localpilot port 3000

# Inspect a process
./localpilot inspect 12345

# Kill by port or PID (with confirmation)
./localpilot kill 3000
./localpilot kill 12345 --force

# List all listening ports
./localpilot list
```

## Commands

| Command | Description |
|---------|-------------|
| `localpilot` | Show the localhost dashboard |
| `localpilot list` | List running processes and listening ports |
| `localpilot port <PORT>` | Find what is using a port |
| `localpilot inspect <PID>` | Inspect a process in detail |
| `localpilot kill <PID\|PORT>` | Safely terminate a process |
| `localpilot kill --force` | Skip confirmation and force kill |

## Example

```bash
$ localpilot port 3000

PORT DOCTOR
Port: 3000
Status: IN USE

Process
────────────────────────────────────
PID             18432
Name            node
CPU             4.2%
Memory          182.0 MB
Started         14:32:01

Command
────────────────────────────────────
npm run dev

Working Directory
────────────────────────────────────
~/Projects/shop/frontend

Git Repository
────────────────────────────────────
shop
Branch: main
Framework: Node.js
```

## Architecture

```
cmd/localpilot/          CLI entry point
internal/
  agent/                 Orchestrates system inspection
  cli/                   Cobra command definitions
  models/                Core data types (Process, Port, Project)
  output/                Terminal formatting
  platform/              OS-specific adapters (Linux, macOS)
  security/              Environment variable masking
```

Platform adapters hide OS-specific process and network inspection behind a common `ProcessProvider` interface.

## Platform Support

| Platform | Status |
|----------|--------|
| Linux    | Supported |
| macOS    | Supported |
| Windows  | Planned |
| WSL      | Planned |

## Security

- **Local-first**: No data leaves your machine
- **Secrets masked**: Sensitive environment variables are redacted by default
- **Confirmation required**: `kill` prompts before terminating unless `--force` is used

## Development

```bash
go mod download
go test ./...
go build -o localpilot ./cmd/localpilot
```

## Roadmap

- [x] v0.1 — Port Doctor (`port`, `inspect`, `kill`, `list`)
- [ ] v0.2 — Docker integration, terminal dashboard, project detection
- [ ] v0.3 — Web dashboard, diagnostics, stale process detection
- [ ] v0.4 — Project profiles, `up`/`down`, Windows/WSL support

## License

MIT — see [LICENSE](LICENSE).
