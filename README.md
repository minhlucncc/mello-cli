# mello

A command-line client for the [Mello](https://mello.mezon.vn) project-management
platform. `mello` manages workspaces, boards, columns, tickets, comments, and
attachments from the terminal, and can mirror a board to a local directory for
offline editing with deliberate, diff-based synchronization back to the server.

- **Single static binary**, no runtime dependencies.
- **Scriptable** — every read command supports `--json`.
- **Local working copy** — clone a board to disk, edit tickets, comments, and
  attachments offline, then review and push a precise set of changes.
- **Safe synchronization** — changes are detected by content hash, and edits made
  on both sides are reported as conflicts rather than silently overwritten.

## Installation

Pre-built binaries are published for Linux, macOS, and Windows on amd64 and
arm64.

### Install script (Linux and macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/minhlucncc/mello-cli/main/install.sh | sh
```

The script downloads the latest release for your platform, verifies its checksum,
and installs it to `/usr/local/bin` (or `~/.local/bin` if that is not writable).
Override the target with `MELLO_INSTALL_DIR`, or pin a version with
`MELLO_VERSION`:

```sh
curl -fsSL https://raw.githubusercontent.com/minhlucncc/mello-cli/main/install.sh \
  | MELLO_VERSION=v1.0.0 sh
```

### Manual download

Download the archive for your platform from the
[releases page](https://github.com/minhlucncc/mello-cli/releases), verify it against
`checksums.txt`, extract it, and place the `mello` binary on your `PATH`. On
Windows, extract the `.zip` and add the directory to your `PATH`.

### Go

```sh
go install github.com/minhlucncc/mello-cli@latest
```

### Container image

```sh
docker run --rm -e MELLO_TOKEN ghcr.io/minhlucncc/mello-cli:latest workspace list
```

### From source

`mello` requires [Go](https://go.dev/dl/) 1.22 or newer and has no other
dependencies.

```sh
git clone https://github.com/minhlucncc/mello-cli
cd mello-cli
make build              # writes ./bin/mello
make install            # installs to $GOBIN
```

## Quick start

```sh
mello auth login                          # store a personal access token
mello workspace use <workspace>           # select a default workspace
mello sync clone -b <board>               # create a workspace and check a board out

# edit ticket files, write comments, add attachments, create new tickets …

mello sync status                         # review pending changes
mello sync push                           # apply them to the server
```

A working copy can hold more than one board. Use `mello init` to create an empty
workspace and check boards out into it:

```sh
mello init                                # create an empty .mello workspace here
mello sync clone -b ROADMAP               # check out a board
mello sync clone -b OPERATIONS            # check out another
mello sync status                         # plan across all boards
```

Most commands act on the only board when there is one, so `-b` is rarely needed.
With several boards, `-b <board>` scopes a command to one, and `mello sync
status`/`pull`/`push` cover them all by default.

## Authentication

Create a personal access token in Mello (Developer → Tokens; tokens have the form
`mello_pat_…`), then authenticate:

```sh
mello auth login                          # prompts for the token (input hidden)
mello auth login --token mello_pat_xxx    # non-interactive
printf '%s' "$TOKEN" | mello auth login --with-token   # from stdin (CI)
mello auth status                         # show the current identity
mello auth logout
```

Credentials are stored per profile in `~/.config/mello/config.json` with file
mode `0600`. See [docs/configuration.md](docs/configuration.md) for profiles and
environment variables.

## Command reference

```
auth        login | logout | status
workspace   list | use <id|name>
board       list | create | view
column      list | create
ticket      list | view | create | edit | move | delete | history
comment     list | add
attachment  list | add | download
member      list
search      <query>
init        (create a local .mello workspace)
new         ticket
sync        clone | status | pull | push | sync
```

Run `mello <command> help` for a command's subcommands and flags. Global flags are
accepted by every command:

| Flag | Description |
|------|-------------|
| `-p`, `--profile <name>` | use a named configuration profile |
| `--base-url <url>` | override the API base URL |
| `--json` | emit machine-readable JSON |
| `--no-color` | disable colored output |

A complete listing is in [docs/commands.md](docs/commands.md).

## Local working copy

A `.mello` directory is a self-contained workspace that mirrors one or more
boards. Commands run from anywhere inside it locate the `.mello` directory by
searching parent directories. Create one with `mello init` or `mello sync clone`.

```
.mello/
  state.json                       # workspace, boards, sync cursors, and per-ticket baselines
  journal.log                      # an audit record of each push
  boards/<board>/tickets/<ticket>/
      ticket.md                    # editable: front matter (fields) + description body
      ticket.json                  # the server baseline used for diffing (do not edit)
      comments/                    # one Markdown file per comment
      attachments/                 # attachment files
```

You edit `ticket.md`, drop new files into `comments/` and `attachments/`, create
tickets with `mello new ticket`, or remove a ticket directory to delete it.
`mello sync status` reports the resulting plan, and `mello sync push` applies it.
Synchronization is incremental and based on content hashes; concurrent edits on
both sides are surfaced as conflicts. The model is described in full in
[docs/working-copy.md](docs/working-copy.md).

## Scripting

Read commands accept `--json` for use in pipelines:

```sh
mello ticket list -b <board> --json | jq -r '.[] | "\(.ticket_code)\t\(.title)"'
mello search "billing" --json | jq length
```

Configuration can be supplied entirely through environment variables
(`MELLO_TOKEN`, `MELLO_BASE_URL`, `MELLO_WORKSPACE`), making the tool suitable for
continuous-integration environments without an interactive login.

## API compatibility

`mello` targets the Mello public API (`/api/v1`). Some endpoints — ticket editing
and deletion, comment retrieval, attachments, history, and the current-user
endpoint — may not be present on every deployment. When an endpoint is
unavailable the affected command reports a clear message and exits non-zero,
rather than failing with an opaque error; unrelated commands are unaffected.

## Development

```sh
make test               # run the test suite
make lint               # gofmt check and go vet
make build              # compile ./bin/mello
```

The codebase is organized as:

```
cmd/                command implementations
internal/mello/     Mello API client and data types
internal/config/    credential and profile store
internal/sync/      local working-copy and synchronization engine
internal/ui/        terminal output helpers
```

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) for
development setup, coding conventions, and the pull-request process.

## License

Released under the MIT License. See [LICENSE](LICENSE).
