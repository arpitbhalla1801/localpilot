# LocalPilot

Your command center for everything running on your machine.

LocalPilot is a developer-first CLI that helps you discover, understand, diagnose, and control everything running on localhost.

📖 Full docs: https://arpitbhalla1801.github.io/localpilot/

No more jumping between `lsof`, `ps`, `netstat`, `docker ps`, and `kill -9`.

## Installation

### Option 1: Homebrew (macOS/Linux, recommended)

```bash
brew install arpitbhalla1801/tap/localpilot
```

This also installs man pages and shell completions automatically.

### Option 2: Download a release binary

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

**Debian / Ubuntu (.deb):** every release also publishes a `.deb` package (with man pages included):

```bash
curl -sLO https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_amd64.deb
sudo dpkg -i localpilot_amd64.deb
```

### Option 3: `go install`

If you already have Go 1.22+ set up:

```bash
go install github.com/arpitbhalla1801/localpilot/cmd/localpilot@latest
```

This installs `localpilot` into `$(go env GOPATH)/bin` — make sure that directory is on your `PATH`.

### Option 4: Build from source

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

# Kill non-interactively and get structured output for scripts
localpilot kill 3000 --force --json

# Free a port (always treats the argument as a port, never a PID)
localpilot free 3000
localpilot free 3000 --force

# List all listening ports
localpilot list

# List ports within a range (e.g. a known dev-server band)
localpilot list --range 3000-4000

# Watch a port or process for changes (Ctrl+C to exit)
localpilot watch 3000
localpilot watch 12345 --interval 500ms

# Scan for port conflicts and misconfigurations
localpilot doctor
```

## Commands

| Command | Description |
|---------|-------------|
| `localpilot` | Show the localhost dashboard |
| `localpilot list` | List running processes and listening ports |
| `localpilot list --range <start>-<end>` | Only show ports within a range, e.g. `3000-4000` |
| `localpilot port <PORT>` | Find what is using a port |
| `localpilot inspect <PID>` | Inspect a process in detail |
| `localpilot kill <PID\|PORT>` | Safely terminate a process |
| `localpilot kill --force` | Skip confirmation and force kill |
| `localpilot kill --force --container` | If the port is Docker-owned, stop the container instead of killing the process |
| `localpilot free <PORT>` | Kill whatever is listening on a port (unambiguous: always a port) |
| `localpilot free --force` | Skip confirmation and force free |
| `localpilot free --force --container` | If the port is Docker-owned, stop the container instead of killing the process |
| `localpilot watch <PID\|PORT>` | Live-updating view of a port or process, until Ctrl+C |
| `localpilot watch --interval <duration>` | Set the refresh interval (default `1s`) |
| `localpilot doctor` | Scan for port conflicts (multiple processes on one port, or projects configured for the same default port) |

## Scripting with `--json`

`list`, `port`, `inspect`, `kill`, `free`, and `doctor` all support `--json` for piping into `jq` or other tooling:

```bash
localpilot list --json | jq '.[].port'
localpilot port 3000 --json | jq '.process.pid'
localpilot inspect 12345 --json | jq '.process.name'
localpilot kill 3000 --force --json | jq '.terminated'
localpilot free 3000 --force --json | jq '.terminated'
localpilot doctor --json | jq '.[].message'
```

Notes for scripting:

- `list --json` always outputs an array — `[]` when empty, never `null`.
- `kill --json` and `free --json` require `--force`: a confirmation prompt on stdout would otherwise corrupt output a script expects to be JSON.
- `kill`/`free` share a result shape: `{"pid": <int>, "port": <int, omitted if killed by PID>, "terminated": <bool>, "cancelled": <bool, omitted if false>, "container": <string, name, omitted unless --container stopped one>, "containerStopped": <bool, omitted if false>}`.

## Docker Awareness

When a port is actually published by a Docker container, `port`, `inspect`, and `list` resolve it to the container's name and image — instead of just showing `docker-proxy` (Linux) or Docker Desktop's backend process (macOS/Windows), which tells you nothing about which container is actually involved:

```
$ localpilot port 18080
...
Docker Container
────────────────────────────────────
Name            my-app
Image           nginx:alpine
ID              2fd6024edf64
```

Detection works automatically whenever the `docker` CLI can reach a daemon, and no-ops silently otherwise — Docker isn't required to use LocalPilot.

**Stopping a container-owned port:** on Docker Desktop (macOS/Windows), the process behind a published port is often a single backend process **shared across every container**, not one per container the way Linux's `docker-proxy` is. Killing it can free the port, but it can also disrupt unrelated containers — so `kill`/`free` behave differently depending on whether anything else is at stake:

- **Only one container is running:** nothing else could be affected either way, so `kill`/`free` behave exactly as they always have — no extra prompts or flags needed, `--force` alone kills the process.
- **Other containers are also running:** killing the process is no longer guaranteed safe. Interactively, you're asked whether to stop the container instead (recommended, and the default answer). Non-interactively (`--force`), this is never guessed — you must pass `--container` explicitly, or the command refuses to proceed:

```bash
localpilot free 3000 --force               # only one container running: kills the process, as always
localpilot free 3000 --force               # multiple containers running: refuses — ambiguous, may affect others
localpilot free 3000 --force --container   # multiple containers running: stops just this one container
```

## WSL Awareness (Windows)

On Windows, a port published from inside WSL shows up locally as owned by `wslrelay.exe` — a single relay process shared across every running distro, telling you nothing about which one (or which process inside it) is actually involved. `port`, `inspect`, and `list` resolve it to the real distro, PID, and process name instead:

```
$ localpilot port 45031
...
WSL Process
────────────────────────────────────
Distro          Ubuntu
PID             1234
Name            node
```

Detection works automatically whenever `wsl.exe` can list running distros, and no-ops silently otherwise. This is read-only detection: stopping the process is out of scope (unlike Docker's `--container`, since a WSL process isn't a separate lifecycle you'd want `kill`/`free` to manage — kill it from inside the distro, or by the resolved PID via `wsl -d <distro> -- kill <PID>`).

## Shell Completion

LocalPilot generates completion scripts for bash, zsh, fish, and PowerShell via `localpilot completion <shell>`.

**Bash**

```bash
# Linux
localpilot completion bash | sudo tee /etc/bash_completion.d/localpilot > /dev/null

# macOS (Homebrew's bash-completion@2)
localpilot completion bash > "$(brew --prefix)/etc/bash_completion.d/localpilot"
```

**Zsh**

```zsh
# Add to a directory on your $fpath, then start a new shell
localpilot completion zsh > "${fpath[1]}/_localpilot"
```

**Fish**

```fish
localpilot completion fish > ~/.config/fish/completions/localpilot.fish
```

**PowerShell**

```powershell
localpilot completion powershell | Out-String | Invoke-Expression

# To persist across sessions, add the line above to your $PROFILE
```

## Man Pages

Release archives (and the Homebrew/apt packages once published) include man pages generated from the real command tree, so they never drift from `--help` text:

```bash
man localpilot
man localpilot-free
```

To generate them locally (e.g. to preview a change before release):

```bash
go run ./tools/gen-man        # writes to ./man
man ./man/localpilot.1
```

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
  platform/              OS-specific adapters (Linux, macOS, Windows)
  security/              Environment variable masking
```

Platform adapters hide OS-specific process and network inspection behind a common `ProcessProvider` interface.

## Platform Support

| Platform | Status |
|----------|--------|
| Linux    | Supported |
| macOS    | Supported |
| Windows  | Supported |
| WSL      | Supported (running natively inside WSL just uses the Linux support above; from Windows, `wslrelay.exe`-owned ports are resolved to their real WSL distro/process — see WSL Awareness) |

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
- [x] v0.2 — Docker container resolution (read-only), terminal dashboard, project detection
- [ ] v0.3 — Web dashboard, diagnostics, stale process detection
- [x] v0.4 — Windows support added, including WSL port resolution

## License

MIT — see [LICENSE](LICENSE).
