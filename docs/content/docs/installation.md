---
title: "Installation"
weight: 1
---

# Installation

## Homebrew (macOS/Linux, recommended)

```bash
brew install arpitbhalla1801/tap/localpilot
```

This also installs man pages and shell completions automatically.

## Download a release binary

Grab the archive for your platform from the [latest release](https://github.com/arpitbhalla1801/localpilot/releases/latest), extract it, and put the binary on your `PATH`.

```bash
# macOS (Apple Silicon)
curl -sL https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_Darwin_arm64.tar.gz | tar -xz

# macOS (Intel)
curl -sL https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_Darwin_x86_64.tar.gz | tar -xz

# Linux (x86_64)
curl -sL https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_Linux_x86_64.tar.gz | tar -xz

# Linux (arm64)
curl -sL https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_Linux_arm64.tar.gz | tar -xz

sudo mv localpilot /usr/local/bin/
```

**Windows:** download `localpilot_Windows_x86_64.zip` (or `_arm64`) from the [releases page](https://github.com/arpitbhalla1801/localpilot/releases/latest), extract it, and add the folder to your `PATH`.

Every release publishes a `checksums.txt` to verify your download with `shasum -a 256 -c checksums.txt`.

## Debian/Ubuntu (.deb)

```bash
curl -sLO https://github.com/arpitbhalla1801/localpilot/releases/latest/download/localpilot_amd64.deb
sudo dpkg -i localpilot_amd64.deb
```

## go install

```bash
go install github.com/arpitbhalla1801/localpilot/cmd/localpilot@latest
```

## Build from source

```bash
git clone https://github.com/arpitbhalla1801/localpilot.git
cd localpilot
go build -o localpilot ./cmd/localpilot
```

## Verify

```bash
localpilot --version
```
