---
title: "Scripting with --json"
weight: 3
---

# Scripting with `--json`

`list`, `port`, `inspect`, `kill`, and `free` all support `--json` for piping into `jq` or other tooling:

```bash
localpilot list --json | jq '.[].port'
localpilot port 3000 --json | jq '.process.pid'
localpilot inspect 12345 --json | jq '.process.name'
localpilot kill 3000 --force --json | jq '.terminated'
localpilot free 3000 --force --json | jq '.terminated'
```

Notes:

- `list --json` always outputs an array — `[]` when empty, never `null`.
- `kill --json` and `free --json` require `--force`: a confirmation prompt on stdout would otherwise corrupt output a script expects to be JSON.
- `kill`/`free` share a result shape: `{"pid": <int>, "port": <int, omitted if killed by PID>, "terminated": <bool>, "cancelled": <bool, omitted if false>}`.
