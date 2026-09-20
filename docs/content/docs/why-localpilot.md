---
title: "Why LocalPilot"
weight: 0
---

# Why LocalPilot

Local development on any given day means juggling several single-purpose tools just to answer "what's on port 3000, and can I kill it?":

| Task | Without LocalPilot | With LocalPilot |
|---|---|---|
| Find what's on a port | `lsof -i :3000` | `localpilot port 3000` |
| Inspect that process | `ps -p <pid> -o command,etime` | `localpilot inspect <pid>` |
| Kill it | `kill -9 <pid>` (no confirmation, easy to hit the wrong PID) | `localpilot kill 3000` (confirms first) |
| See what's holding every dev port | some combination of `lsof`, `netstat`, `docker ps` | `localpilot list` |
| Know *which project* owns a stray process | manual `cd` + `git branch` guesswork | shown automatically (repo, branch, framework) |

LocalPilot is one static binary that answers all of these, with:

- **Project context out of the box** — every process is enriched with its git repository, branch, and detected framework (Node.js, Go, Rust, Python, ...), so a stray port is traceable to the project holding it, not just a bare PID.
- **Guardrails on destructive actions** — `kill`/`free` prompt for confirmation, and refuse to run non-interactively without `--force`, so a script can't silently kill something because a TTY prompt went unanswered.
- **Secrets masked by default** — environment variables shown by `inspect` are redacted for common secret patterns (API keys, tokens, database URLs) before they ever hit your terminal.
- **`--json` everywhere** — every read/write command has a structured output mode for scripts and CI, not just humans.
- **No runtime dependencies** — a single Go binary per platform (Linux, macOS, Windows), installable via Homebrew, `.deb`, or `go install`.
