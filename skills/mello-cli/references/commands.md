# Command reference

Source of truth: the command tree in `cmd/root.go`. Run `mello <command> help`
for any group's subcommands.

## Global flags (accepted by every command)

| Flag | Meaning |
|------|---------|
| `-p`, `--profile <name>` | use a named configuration profile |
| `--base-url <url>` | override the API base URL |
| `--json` | emit machine-readable JSON (read commands) |
| `--no-color` | disable ANSI color |
| `-v`, `--version` | print version info (top level) |

Notes that matter when driving the CLI:

- **Flags may appear after positional arguments** — `mello ticket move PROJ-1
  --column COL` works as well as flags-first.
- **Board-scoped commands default to the working board** (set by `mello use`),
  so `-b` is usually unnecessary inside a `.mello` workspace.
- **Exit codes:** `0` success · `1` runtime error · `2` invalid usage.

## auth

| Command | Description |
|---------|-------------|
| `auth login [--token <pat>] [--with-token]` | Store a `mello_pat_…` token. No `--token` ⇒ hidden prompt; `--with-token` ⇒ read from stdin. Auto-selects the workspace when the token has exactly one. |
| `auth logout` | Remove the active profile's token. |
| `auth status` | Show identity, profile, base URL, and workspace. |

## workspace

| Command | Description |
|---------|-------------|
| `workspace list [--json]` | List accessible workspaces. |
| `workspace use <id\|name>` | Set the default workspace for the active profile. |

## board

| Command | Description |
|---------|-------------|
| `board list [-w <ws>] [--json]` | List boards. Inside a workspace ⇒ that workspace's boards; otherwise across all accessible workspaces (with a WORKSPACE column). |
| `board create <name> [--code <code>] [-w <ws>]` | Create a board. |
| `board view [<board>] [-b <board>] [--json]` | Show a board's columns and ticket counts (defaults to the working board). |
| `board use <board>` | Alias of `mello use` — set the working board. |

## column

| Command | Description |
|---------|-------------|
| `column list [-b <board>] [--json]` | List a board's columns (id, name, position). |
| `column create <name> [-b <board>]` | Add a column to a board. |

## ticket

| Command | Description |
|---------|-------------|
| `ticket list [-b <board>] [--mine] [--assignee <id\|me>] [--column <name\|id>] [--status <s>] [--json]` | List tickets on the (working) board. Shows ticket code, column, members, status, title. `--mine`/`--assignee me` filter by membership. Full records (status/members) are sourced from the workspace tickets endpoint when available. |
| `ticket view <id\|code> [--no-comments] [--comments <N>] [--history <N>] [--json]` | Full detail: status, members, labels, column, board, dates, description, last-N comments (default 5), attachments, last-N history (default 5). `--json` dumps the **raw server payload**. A code (e.g. `PROJ-12`) is resolved to its id via the working board. |
| `ticket create -t <title> [-b <board>] [-c <column-name>] [-d <desc>\|--body-file <f>] [--json]` | Create a ticket. Defaults to the working board and its **first column**; `-c` takes a column **name**. |
| `ticket edit <id> [-t <title>] [-d <desc>\|--body-file <f>] [--status <s>] [--assignee <id\|me>] [--labels a,b]` | Update only the fields you pass. `--assignee me` assigns yourself. Optional endpoint; degrades clearly if unsupported. |
| `ticket move <id> --column <column-id> [--position <n>]` | Move a ticket. `--column` is a column **id** — get ids from `mello column list`. |
| `ticket delete <id> [-y]` | Delete the ticket **on the server** (confirmation unless `-y`). Distinct from `untrack`. |
| `ticket history <id> [--json]` | Show a ticket's activity history. Optional endpoint. |

## comment

| Command | Description |
|---------|-------------|
| `comment list <ticket> [--json]` | List a ticket's comments. |
| `comment add <ticket> [-b <body>\|--body-file <f>]` | Add a comment; the body may also be piped on stdin. |

## attachment

| Command | Description |
|---------|-------------|
| `attachment list <ticket> [--json]` | List a ticket's attachments. |
| `attachment add <ticket> <file>...` | Upload one or more files. |
| `attachment download <ticket> [--dir <d>]` | Download a ticket's attachments. |

Attachments are an optional endpoint; commands degrade with a clear message when
the instance doesn't support them.

## member

| Command | Description |
|---------|-------------|
| `member list [-w <ws>] [--json]` | List workspace members (user id, name, email, role). |

## search

| Command | Description |
|---------|-------------|
| `search <query...> [-w <ws>] [--json]` | Full-text ticket search. Inside a workspace ⇒ that workspace; otherwise across all. |

## Local workspace & working set

| Command | Description |
|---------|-------------|
| `init [<dir>] [-w <ws>]` | Create an empty `.mello` workspace (default current dir). Requires nothing. |
| `use <board>` | Resolve a board (across your workspaces), bind its workspace, and make it the **working board**. Downloads no tickets. |
| `clone <board> [-w <ws>] [--dir <d>]` | Like `use` but also mirrors **all** of the board's tickets locally. Board may be positional or `-b`. |
| `pull [<ticket>] [-b <board>] [--all]` | No arg ⇒ refresh the working set. `<ticket>` ⇒ pull one ticket into the working set. `--all` ⇒ mirror the whole board. |
| `status [-b <board>] [--all] [--remote] [--json]` | Show the plan of pending local changes: `+` create, `~` update/move/comment/attach. `--remote` also checks the server for `↓` drift / `!` conflicts. Defaults to the working board; `--all` spans every board. |
| `push [-b <board>] [--all] [--dry-run] [--force]` | Apply local changes to the server. `--dry-run` previews; `--force` pushes conflicts (local wins). |
| `new ticket -t <title> --column <name> [-b <board>] [-d <desc>\|--body-file <f>]` | Scaffold a local ticket in the working set; created on the server on the next `push`. |
| `untrack <ticket>... [-b <board>] [--all]` | Drop tickets from the working set locally. **Does not delete on the server** (use `ticket delete` for that). |
| `sync clone\|status\|pull\|push\|sync` | The same verbs grouped under `sync`; `sync sync [--force]` does pull-then-push (reconcile). |

## cache

| Command | Description |
|---------|-------------|
| `cache clear` | Delete the local metadata cache (workspaces/boards/members/columns). |

## version

`version` (or `-v`/`--version`) prints version, commit, build date, and
Go/platform.
