---
title: "LocalPilot"
type: docs
bookFlatSection: true
weight: 1
---

# LocalPilot

**Your command center for everything running on your machine.**

LocalPilot is a developer-first CLI that helps you discover, understand, diagnose, and control everything running on localhost — no more jumping between `lsof`, `ps`, `netstat`, `docker ps`, and `kill -9`.

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

## Why LocalPilot

- **One tool, not five.** `port`, `inspect`, `kill`, `free`, `watch`, `list` replace a chain of `lsof -i :3000`, `ps -p <pid>`, `kill -9`.
- **Project-aware.** Every process is enriched with its git repo, branch, and detected framework — you see *which project* is holding a port, not just a PID.
- **Safe by default.** `kill`/`free` prompt before terminating, refuse to prompt non-interactively (no accidental kills from scripts), and mask secrets in environment output.
- **Scriptable.** Every command supports `--json` for piping into `jq` or CI.
- **Cross-platform.** Linux, macOS, and Windows, one binary, no runtime dependency.

## Get started

- [Installation](/docs/installation/)
- [Quickstart](/docs/quickstart/)
- [Command reference](/commands/)
