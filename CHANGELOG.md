# Changelog

All notable changes to this project are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Decode tickets whose `labels` are returned as objects (the live API shape), not
  only as plain strings — previously this failed `use`, `pull`, and `ticket list`.
- Flags are now accepted after positional arguments (for example
  `mello ticket move <id> --column <c>`); previously such flags were silently
  ignored.

### Changed

- The local working copy is now a lazy *working set*: `mello use <board>` sets the
  working board without downloading any tickets. Browse the board live with
  `mello ticket list`, pull individual tickets with `mello pull <ticket>` (or
  mirror everything with `mello pull --all`), or create tickets locally.
- Removing a ticket from the working set is safe: deleting its folder (or running
  `mello untrack`) stops tracking it locally and never deletes it on the server.
  Use `mello ticket delete` to remove a ticket on the server.

### Added

- `auth login` selects the workspace automatically when the token has access to
  exactly one, so the personal access token is the only setup required. The CLI
  also runs entirely from `MELLO_TOKEN`/`MELLO_WORKSPACE` without `auth login`.
- `untrack` command to drop tickets from the working set (keeping them on the
  server).
- `init` command to create an empty local `.mello` workspace, and support for
  checking multiple boards out into one workspace. Commands default to the sole
  board, accept `-b <board>` to scope to one, and span all boards for
  status/pull/push.
- Authentication with personal access tokens, stored per profile in
  `~/.config/mello/config.json` (`auth login`, `logout`, `status`).
- Resource commands for workspaces, boards, columns, tickets, comments,
  attachments, members, and full-text search.
- Local working copy (`sync clone`) that mirrors a board under `.mello/`.
- Incremental, hash-based synchronization: `sync status`, `sync pull`,
  `sync push`, and `sync sync` (pull then push).
- Creation, modification, movement, and deletion of tickets through the working
  copy, including `new ticket` to scaffold a local ticket.
- Conflict detection when a ticket has changed on both the local and remote
  sides, with an opt-in `--force` to override.
- Per-push audit trail (`journal.log`) and a monotonic state serial.
- `--json` output for read commands and environment-variable configuration for
  non-interactive use.
- `version` command and `--version` flag reporting version, commit, build date,
  and Go/platform information.
- Cross-platform release builds (Linux, macOS, Windows; amd64 and arm64) via
  GoReleaser, published with checksums to GitHub releases.
- `install.sh` for one-line installation on Linux and macOS.
- Container image and `Dockerfile`.
- Continuous-integration and release GitHub Actions workflows.
