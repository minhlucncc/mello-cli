# Command reference

Global flags accepted by every command:

| Flag | Description |
|------|-------------|
| `-p`, `--profile <name>` | use a named configuration profile |
| `--base-url <url>` | override the API base URL |
| `--json` | emit machine-readable JSON |
| `--no-color` | disable colored output |

## auth

| Command | Description |
|---------|-------------|
| `auth login [--token <pat>] [--with-token]` | Store a token. Without `--token`, the token is read interactively (input hidden) or, with `--with-token`, from standard input. |
| `auth logout` | Remove the active profile's token. |
| `auth status` | Show the current identity, profile, and base URL. |

## workspace

| Command | Description |
|---------|-------------|
| `workspace list` | List accessible workspaces. |
| `workspace use <id\|name>` | Set the default workspace for the active profile. |

## board

| Command | Description |
|---------|-------------|
| `board list [-w <workspace>]` | List boards in a workspace. |
| `board create <name> [--code <code>] [-w <workspace>]` | Create a board. |
| `board view <board>` | Show a board's columns and ticket counts. |

## column

| Command | Description |
|---------|-------------|
| `column list -b <board>` | List a board's columns. |
| `column create -b <board> <name>` | Add a column to a board. |

## ticket

| Command | Description |
|---------|-------------|
| `ticket list -b <board> [--column <c>] [--assignee <a>]` | List tickets on a board. |
| `ticket view <id> [--no-comments]` | Show a ticket and its comments. |
| `ticket create -c <column> -t <title> [-d <desc>\|--body-file <f>]` | Create a ticket. |
| `ticket edit <id> [-t][-d][--status][--assignee][--labels a,b]` | Update fields. |
| `ticket move <id> --column <c> [--position <n>]` | Move a ticket. |
| `ticket delete <id> [-y]` | Delete a ticket. |
| `ticket history <id>` | Show a ticket's activity history. |

## comment

| Command | Description |
|---------|-------------|
| `comment list <ticket>` | List a ticket's comments. |
| `comment add <ticket> [-b <body>\|--body-file <f>]` | Add a comment; the body may also be supplied on standard input. |

## attachment

| Command | Description |
|---------|-------------|
| `attachment list <ticket>` | List a ticket's attachments. |
| `attachment add <ticket> <file>...` | Upload one or more files. |
| `attachment download <ticket> [--dir <d>]` | Download a ticket's attachments. |

## member

| Command | Description |
|---------|-------------|
| `member list [-w <workspace>]` | List workspace members. |

## search

| Command | Description |
|---------|-------------|
| `search <query> [-w <workspace>]` | Full-text search of tickets. |

## new

| Command | Description |
|---------|-------------|
| `new ticket --column <name> -t <title> [-d <desc>\|--body-file <f>]` | Scaffold a ticket in the local working copy; it is created on the server on the next push. |

## sync

| Command | Description |
|---------|-------------|
| `sync clone -b <board> [--dir <d>]` | Mirror a board into a local working copy. |
| `sync status [--remote]` | Show the plan of pending changes. With `--remote`, also fetch the server to detect drift and conflicts. |
| `sync pull` | Apply remote changes to the working copy. |
| `sync push [--dry-run] [--force]` | Apply local changes to the server. |
| `sync sync [--force]` | Pull, then push. |

See [working-copy.md](working-copy.md) for the synchronization model.

## Exit status

| Code | Meaning |
|------|---------|
| `0` | success |
| `1` | a runtime error occurred (the message is printed to standard error) |
| `2` | invalid command-line usage |
