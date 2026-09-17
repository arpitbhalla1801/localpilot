# LocalPilot

Your command center for everything running on your machine.

LocalPilot is a developer-first CLI that helps you discover, understand, diagnose, and control everything running on localhost.

No more jumping between `lsof`, `ps`, `netstat`, `docker ps`, and `kill -9`.

## Installation

### Option 1: Download a release binary (recommended)

Grab the archive for your platform from the [latest release](https://github.com/arpitbhalla1801/localpilot/releases/latest), then extract it and put the binary on your `PATH`.

**macOS / Linux:**

```bash
# macOS (Apple Silicon)
curl -sL https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_Darwin_arm64.tar.gz | tar -xz

# macOS (Intel)
curl -sL https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_Darwin_x86_64.tar.gz | tar -xz

# Linux (x86_64)
curl -sL https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_Linux_x86_64.tar.gz | tar -xz

# Linux (arm64)
curl -sL https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_Linux_arm64.tar.gz | tar -xz

# then move it onto your PATH
sudo mv localpilot /usr/local/bin/
```

**Windows:** download `localpilot_Windows_x86_64.zip` (or `_arm64` on ARM) from the [releases page](https://github.com/arpitbhalla1801/localpilot/releases/latest), extract it, and add the folder to your `PATH`.

Every release also publishes a `checksums.txt` — verify your download with `shasum -a 256 -c checksums.txt` (macOS/Linux) if you want to confirm integrity.

### Option 2: `go install`

If you already have Go 1.22+ set up:

```bash
go install github.com/arpitbhalla1801/localpilot/cmd/localpilot@latest
```

This installs `localpilot` into `$(go env GOPATH)/bin` — make sure that directory is on your `PATH`.

### Option 3: Build from source

```bash
git clone https://github.com/arpitbhalla1801/localpilot.git
cd localpilot
go build -o localpilot ./cmd/localpilot
```

### Verify it's installed

```bash
localpilot --version
```

## Quick Start

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
localpilot free 3000 --force

# List all listening ports
localpilot list
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
| `localpilot free <PORT>` | Kill whatever is listening on a port (unambiguous: always a port) |
| `localpilot free --force` | Skip confirmation and force free |

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
| Windows  | Supported |
| WSL      | Planned |

## Security

- **Local-first**: No data leaves your machine
- **Secrets masked**: Sensitive environment variables are redacted by default
- **Confirmation required**: `kill` and `free` prompt before terminating unless `--force` is used

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
- [x] v0.4 — Windows support added (WSL planned)

## License

MIT — see [LICENSE](LICENSE).
