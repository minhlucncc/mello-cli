# Configuration, storage & exit codes

## Authentication (two backends)

The CLI auto-detects the backend from the token type:

| Token | Backend (base URL) | Notes |
|-------|--------------------|-------|
| `mello_pat_…` (personal access token) | public `https://mello.mezon.vn/api/v1` | official; scopes `read`/`write`; currently limited data |
| session **JWT** (`eyJ…`) or **refresh token** | internal `https://mello.mezon.vn/api` | full data — members, attachments, comments, activity/history |

```sh
# Public API key
mello auth login --token mello_pat_xxx
printf '%s' "$KEY" | mello auth login --with-token   # CI / piped

# Session — refresh token (auto-renews; recommended).
# Browser: DevTools → Application → Cookies → mello.mezon.vn → refresh_token
mello auth login --refresh-token <refresh_token>

# Session — access token (~1h, no auto-renew).
# Browser: DevTools → Application → Local Storage → mello.mezon.vn → mello.access_token
mello auth login --token <jwt>

mello auth status      # identity, profile, base URL, workspace
mello auth logout
```

### Refresh-token auto-renewal

`--refresh-token` calls `POST /api/auth/refresh`, stores the resulting access
token, and renews it automatically when it expires. **Refresh tokens are
single-use and rotate** — the server returns a new one each time, which the CLI
persists. Implication: don't share one session between the CLI and the browser at
the same time; whichever refreshes second is logged out. A `forbidden` error
means insufficient scope; an `unauthorized`/expired refresh error means grab a
fresh `refresh_token` from the browser cookie.

The user's id is captured at login (from the JWT `sub` or `/me`) so `me` /
`--mine` resolve without an extra request.

## Profiles & config file

Credentials live in `~/.config/mello/config.json` (mode `0600`), keyed by named
profile:

```json
{
  "current": "default",
  "profiles": {
    "default": {
      "base_url": "https://mello.mezon.vn/api",
      "token": "<access token>",
      "refresh_token": "<rotated each renewal>",
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
