# Changelog

All notable changes to this project are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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
