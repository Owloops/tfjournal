<div align="center">

# tfjournal

Record Terraform runs with git context, timing, and resource events.

[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Owloops/tfjournal)](https://github.com/Owloops/tfjournal/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/Owloops/tfjournal)](https://goreportcard.com/report/github.com/Owloops/tfjournal)
[![CI/CD](https://github.com/Owloops/tfjournal/actions/workflows/release.yml/badge.svg)](https://github.com/Owloops/tfjournal/actions/workflows/release.yml)

</div>

## Features

<table>
<tr>
<td width="50%">

**Run recording**

Wraps terraform/tofu/terragrunt commands and captures output, timing, exit codes.

</td>
<td width="50%">

**Git and CI context**

Automatically records commit, branch, user, and CI provider info.

</td>
</tr>
<tr>
<td width="50%">

**Resource timeline**

Parses resource-level events and displays them as a gantt chart.

</td>
<td width="50%">

**S3 sync**

Optional S3 backend for sharing history across machines or teams.

</td>
</tr>
</table>

## Installation

### Homebrew

```bash
brew install Owloops/tap/tfjournal
```

### Install script

```bash
curl -sSL https://raw.githubusercontent.com/Owloops/tfjournal/main/install.sh | bash
```

### Go

```bash
go install github.com/Owloops/tfjournal@latest
```

### Build from source

```bash
git clone https://github.com/Owloops/tfjournal.git
cd tfjournal
make build
```

## Usage

```bash
# Record a terraform run
tfjournal -- terraform apply

# Set workspace name manually
tfjournal -w prod -- tofu plan

# Works with terragrunt
tfjournal -- terragrunt run-all apply

# Open TUI
tfjournal

# List runs
tfjournal list
tfjournal list --since 7d --failed

# Show run details
tfjournal show run_abc123
tfjournal show run_abc123 --output
```

### Shell Aliases

```bash
alias tf='tfjournal -- terraform'
alias tofu='tfjournal -- tofu'
alias tg='tfjournal -- terragrunt'
```

## TUI

Run `tfjournal` without arguments:

```
┌───────────────┬─────────────────────────────────────────────────┐
│ Search        │ q:quit j/k:nav s:sync /:search ?:hide           │
├───────────────┼─────────────────────────────────────────────────┤
│ Runs (47)     │ ▶ d:Details │ e:Events │ t:Timeline │ o:Output  │
│               ├─────────────────────────────────────────────────┤
│ ✓ 01-23 +1~0  │ Run:        run_abc123                          │
│ ✗ 01-22 +0~1  │ Workspace:  production/alb                      │
│ ✓ 01-22 +3~0  │ Status:     ✓ SUCCESS                           │
│               │ Duration:   2m34s                               │
│               │ User:       papuna                              │
│               │ Git:        abc123 (main)                       │
└───────────────┴─────────────────────────────────────────────────┘
```

### Views

| Key | View | Description |
|-----|------|-------------|
| `d` | Details | Run metadata, git info, resource list |
| `e` | Events | Table of resource changes with duration |
| `t` | Timeline | Gantt chart of resource execution |
| `o` | Output | Captured command output |

### Keys

| Key | Action |
|-----|--------|
| `j/k` | Navigate runs |
| `g/G` | First/last run |
| `Enter` | Focus content panel for scrolling |
| `Escape` | Back to runs list, clear search |
| `/` | Search |
| `s` | Sync to S3 |
| `?` | Toggle help |
| `q` | Quit |

### Sync Indicators

With S3 configured:

- `✓` local and S3
- `↓` local only
- `↑` S3 only

## S3 Backend

```bash
export TFJOURNAL_S3_BUCKET=my-tfjournal
export TFJOURNAL_S3_REGION=us-east-1
export TFJOURNAL_S3_PREFIX=team-a  # optional
```

Without these variables, tfjournal uses local storage only.

Writes go to local storage first, then upload to S3 in the background. The TUI loads local runs immediately and fetches S3 runs in the background.

### Team Usage

Share a single S3 bucket across teams using different prefixes:

```bash
# Team A
export TFJOURNAL_S3_PREFIX=team-a

# Team B
export TFJOURNAL_S3_PREFIX=team-b
```

Each team sees only their runs. Filter by user with `tfjournal list --user alice` or search by username in TUI.

## Data

Each run records:

```json
{
  "id": "run_abc123",
  "workspace": "production/alb",
  "timestamp": "2025-01-23T10:30:00Z",
  "duration_ms": 154000,
  "status": "success",
  "program": "terraform",
  "user": "papuna",
  "git": {
    "commit": "abc123",
    "branch": "main",
    "dirty": false
  },
  "ci": {
    "provider": "github-actions",
    "actor": "papuna"
  },
  "changes": { "add": 2, "change": 0, "destroy": 0 },
  "resources": [
    {
      "address": "aws_instance.web",
      "action": "create",
      "duration_ms": 30000
    }
  ]
}
```

### Storage Location

```
~/.local/share/tfjournal/
├── runs/
│   └── run_abc123.json
└── outputs/
    └── run_abc123.txt
```

Override with `TFJOURNAL_STORAGE_PATH`.

## CI Detection

| Provider | Detection | Actor |
|----------|-----------|-------|
| GitHub Actions | `GITHUB_ACTIONS=true` | `GITHUB_ACTOR` |
| GitLab CI | `GITLAB_CI=true` | `GITLAB_USER_LOGIN` |

## CLI Reference

### list

```bash
tfjournal list [workspace-pattern] [flags]

Flags:
  --since string   Filter by time (7d, 24h)
  --user string    Filter by user
  --failed         Only failed runs
  -n, --limit int  Max runs (default: 20)
  --json           JSON output
```

### show

```bash
tfjournal show <run-id> [flags]

Flags:
  --output   Show captured output
  --json     JSON output
```

## License

[MIT](LICENSE)
