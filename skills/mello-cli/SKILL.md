---
name: mello-cli
description: >-
  Use when the user wants to work with the Mello project-management platform
  (workspaces, boards, columns, tickets, comments, attachments) through the
  `mello` command-line tool — e.g. listing/viewing/creating/editing/moving
  tickets, finding tickets assigned to them, commenting, attaching files,
  searching, or syncing a board to a local working copy. Covers authentication,
  configuration, the workspace/working-board/working-set model, and the full
  command surface.
---

# Working with the `mello` CLI

`mello` is a single-binary command-line client for the Mello project-management
platform (`https://mello.mezon.vn`, REST API at `/api/v1`). Use it to manage
workspaces, boards, columns, tickets, comments, and attachments from the
terminal, and to mirror a board to a local directory for offline edits that are
synchronized back.

Confirm it is installed before using it:

```sh
mello version        # prints version/commit/build info
mello --help         # lists all commands
```

If `mello` is not found, it can be built from this repo (`make build`, Go 1.22+)
or installed via `./install.sh`. See `references/configuration.md`.

## First: authenticate (two backends)

The CLI talks to either of two backends, **auto-detected from the token**:

| Token | Backend | Data |
|-------|---------|------|
| **personal access token** `mello_pat_…` | public `/api/v1` | official, but currently limited |
| **session JWT / refresh token** (from the web app) | internal `/api` | **full** — members, **attachments**, comments, activity/history |

For complete data (attachments, members, history) use the **session** auth.

```sh
# Public API key
mello auth login --token mello_pat_xxx

# Session: refresh token (best — auto-renews, ~30 days). From the browser:
#   DevTools → Application → Cookies → mello.mezon.vn → refresh_token
mello auth login --refresh-token <refresh_token>

# Session: access token (quick, ~1h, no auto-renew). From the browser:
#   DevTools → Application → Local Storage → mello.mezon.vn → mello.access_token
mello auth login --token <jwt>

mello auth status                      # who am I? which workspace/base URL?
```

- With `--refresh-token`, the CLI exchanges it for an access token and
  **auto-renews** on every command (refresh tokens are single-use and rotate; the
  CLI persists the rotated one). Log in once, stay logged in.
- A token with one workspace auto-selects it; with several, the CLI prompts
  (interactive) or you pass `-w <id>`.
- Headless/CI: set `MELLO_TOKEN` (+ `MELLO_WORKSPACE`, and `MELLO_BASE_URL` if
  using the internal API) — no `auth login` needed.
- **Always check `mello auth status` first** when unsure.

## Mental model (read this before driving the CLI)

| Term | What it is |
|------|-----------|
| **workspace** | a `.mello/` working directory, created by `mello init`. It binds a Mello workspace and holds local state + cache. |
| **working board** | the board set with `mello use <board>`. Once set, board-scoped commands default to it — no `-b`/`-w` needed. |
| **remote set** | the live tickets on the server. Browse with `mello ticket list` and `mello ticket view`. |
| **working set** | the local subset of tickets you have pulled (`mello pull <ticket>`) or created (`mello new ticket`). `status`/`push` operate on this set only. |

Key safety rule: removing a ticket from the working set is **not** a remote
delete. `mello untrack <ticket>` (or deleting its folder) only stops tracking it
locally; the ticket stays on the server. To delete on the server, use
`mello ticket delete <id>`.

## The day-to-day workflow

```sh
mello auth login                 # once
mello init                       # create a .mello workspace in the current dir (needs nothing)
mello use ROAD                   # pick the board to work on (by code/slug; resolved across your workspaces)

mello ticket list                # browse the board (the remote set)
mello ticket list --mine         # only tickets assigned to you
mello ticket view PROJ-12         # full detail of one ticket

mello pull PROJ-12                # pull a ticket into the working set to edit it
mello new ticket -t "Write spec"  # or create one locally

# edit .mello/boards/<board>/tickets/<ticket>/ticket.md, or use `mello ticket edit`
# drop files into that ticket's comments/ and attachments/ folders
mello status                     # review pending local changes
mello push -m "Updated per review"   # apply to the server; -m posts a note comment
mello pull                       # later: refresh the working set
mello untrack PROJ-12             # done with it → drop from the working set (kept on server)
```

On `push`: edited fields → `PATCH`; column change → move; new files in
`comments/` → posted; new files in `attachments/` → **uploaded**; `-m/--comment`
posts an extra comment on each changed ticket announcing the push.

You do not have to use the local working set at all — many tasks are one-shot
live commands (`ticket create`, `ticket edit`, `ticket move`, `comment add`,
`search`). The working set is for offline/batch editing.

## Most common commands

```sh
mello ticket list [--mine] [--assignee me] [--column "In Progress"] [--status open]
mello ticket view <id|code> [--json]      # full detail; --json = raw server payload
mello ticket create -t "Title" [-c "Column"] [-d "desc"]
mello ticket edit <id> [--title] [--status] [--assignee me] [--labels a,b]
mello ticket move <id> --column "Done"
mello comment add <id> -b "message"        # or pipe the body on stdin
mello search "query"
mello board list                           # all your boards (or the current workspace's)
mello member list
```

## Rules of thumb for an agent

- **Parse with `--json`.** Every read command supports `--json`; use it whenever
  you need to read values programmatically rather than scraping the table.
  `mello ticket view <id> --json` returns the raw server object (all fields).
- **Run inside the `.mello` workspace** so the working board applies and you can
  omit `-b`/`-w`. Outside a workspace, pass `-b <board>` / `-w <workspace>`.
- **`me` / `--mine`** resolve to the authenticated user. Tickets can have
  **multiple members (assignees)**; filters match membership.
- **Flags work in any position** — `mello ticket move PROJ-1 --column Done` is
  fine (flags after the id).
- **Backend matters for completeness.** The internal `/api` (session auth) has
  members, attachments, comments, and history; the public `/api/v1` (API key) is
  more limited. If attachments/history look empty, you're probably on the public
  API — re-auth with a session token (see `references/usecases/setup-and-auth.md`).
- **Optional endpoints degrade gracefully.** When an endpoint isn't supported the
  command prints a clear "not supported" message and exits non-zero — don't retry
  blindly; report it.
- **Writes need a `write`-scoped token / a valid session.** A `forbidden` error
  means insufficient scope; `unauthorized` means re-authenticate.
- **Exit codes:** `0` success · `1` runtime error (message on stderr) · `2`
  invalid usage.
- **Discover** with `mello <command> help` (e.g. `mello ticket help`).

## Use cases (one file each)

Open the relevant file for step-by-step commands:

- `references/usecases/setup-and-auth.md` — first-time setup & authentication.
- `references/usecases/find-my-tickets.md` — list tickets assigned to me; filters.
- `references/usecases/view-ticket.md` — inspect a ticket in full.
- `references/usecases/create-ticket.md` — create a ticket.
- `references/usecases/edit-move-assign.md` — edit fields, move columns, assign.
- `references/usecases/comments-and-attachments.md` — comment and attach files.
- `references/usecases/search.md` — search tickets.
- `references/usecases/offline-work-loop.md` — the local working-set loop.
- `references/usecases/conflicts.md` — resolve local/remote conflicts.
- `references/usecases/multiple-boards.md` — several boards in one workspace.
- `references/usecases/scripting.md` — scripting with `--json`.

## Reference

- `references/commands.md` — every command and flag, with examples.
- `references/configuration.md` — auth, profiles, env vars, the local cache,
  `.mello/` layout, and exit codes.
