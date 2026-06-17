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

## First: authenticate

A Mello **personal access token** (`mello_pat_…`) is the only credential needed;
everything the CLI can do is authorized by that token's scopes (`read` /
`write`).

```sh
mello auth login                       # prompts for the token (hidden input)
mello auth login --token mello_pat_xxx # non-interactive
printf '%s' "$TOKEN" | mello auth login --with-token   # CI / piped
mello auth status                      # who am I? which workspace/base URL?
```

- A token with access to a single workspace auto-selects it; with several, the
  CLI prompts (interactive) or you pass `-w <id>`.
- It can also run with no `auth login` by setting `MELLO_TOKEN` (and
  `MELLO_WORKSPACE` if needed) — handy for scripts/CI.
- **Always check `mello auth status` first** when unsure; "not logged in" means
  authenticate before other commands.

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
mello status                     # review pending local changes
mello push                       # apply them to the server
mello pull                       # later: refresh the working set
mello untrack PROJ-12             # done with it → drop from the working set (kept on server)
```

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
- **Optional endpoints degrade gracefully.** If an instance lacks attachments,
  history, or ticket-edit endpoints, the command prints a clear "not supported"
  message and exits non-zero — don't retry blindly; report it.
- **Writes need a `write`-scoped token.** A `forbidden` error means insufficient
  scope.
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
