# Configuration, storage & exit codes

## Authentication

A Mello personal access token (`mello_pat_…`) is the only credential. Scopes:
`read` (browse) and `write` (create/update). A `forbidden` error means the token
lacks the needed scope.

```sh
mello auth login                       # hidden prompt
mello auth login --token mello_pat_xxx # non-interactive
printf '%s' "$TOKEN" | mello auth login --with-token
mello auth status                      # identity, profile, base URL, workspace
mello auth logout
```

The user's id is captured at login so `me` / `--mine` resolve without an extra
request.

## Profiles & config file

Credentials live in `~/.config/mello/config.json` (mode `0600`), keyed by named
profile:

```json
{
  "current": "default",
  "profiles": {
    "default": {
      "base_url": "https://mello.mezon.vn/api/v1",
      "token": "mello_pat_…",
      "workspace_id": "…",
      "user_id": "…"
    }
  }
}
```

Select a profile per command with `-p <name>` / `--profile <name>`.

## Environment variables

| Variable | Effect |
|----------|--------|
| `MELLO_TOKEN` | token; overrides the stored one (run without `auth login`) |
| `MELLO_BASE_URL` | API base URL (default `https://mello.mezon.vn/api/v1`) |
| `MELLO_WORKSPACE` | default workspace id |
| `MELLO_PROFILE` | active profile name |
| `MELLO_CONFIG_DIR` | config directory (default `~/.config/mello`) |
| `MELLO_CACHE_TTL` | metadata cache TTL in seconds (default 300) |
| `MELLO_NO_CACHE` | set to disable the metadata cache |
| `NO_COLOR` | set to disable colored output |

Resolution order for a setting: command flag → environment variable → active
profile → built-in default.

## The local metadata cache

Slow-changing metadata (workspaces, boards, members, columns) is cached to avoid
re-fetching and to provide fast id→name / code→id lookups (e.g. resolving
assignee ids to member names).

- Location: `.mello/cache/` inside a workspace, else `~/.config/mello/cache/`.
- TTL: 5 minutes by default (`MELLO_CACHE_TTL` seconds). Disable with
  `MELLO_NO_CACHE`.
- Invalidated automatically when you create a board/column.
- Wipe it with `mello cache clear`.

## The `.mello/` workspace layout

Created by `mello init` (or `mello use`/`clone`). Commands find it by searching
parent directories, like `git`.

```
.mello/
  state.json                       # tracked workspace, boards, sync cursors, baselines
  journal.log                      # an audit line per push
  cache/                           # metadata cache
  boards/<board>/tickets/<ticket>/
      ticket.md                    # editable: front matter (fields) + description body
      ticket.json                  # server baseline used for diffing (do not edit)
      ticket.remote.json           # written only on a conflict (the server's version)
      comments/                    # one Markdown file per comment (drop a new file to post)
      attachments/                 # attachment files (drop a new file to upload)
```

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | success |
| `1` | a runtime error occurred (message on stderr) |
| `2` | invalid command-line usage |

## Installation

- Pre-built: `curl -fsSL <repo>/install.sh | sh` (Linux/macOS), or download a
  release archive and put `mello` on `PATH`.
- From source: Go 1.22+, then `make build` (writes `./bin/mello`) or
  `make install`, or `go install github.com/minhlucncc/mello-cli@latest`.
