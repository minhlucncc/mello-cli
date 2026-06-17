# Contributing

Thank you for your interest in improving `mello`. This document describes how to
set up a development environment and the conventions used in the project.

## Requirements

- [Go](https://go.dev/dl/) 1.22 or newer.
- No other dependencies; the project uses only the Go standard library.

## Building and testing

```sh
make build      # compile ./bin/mello
make test       # run the full test suite
make lint       # gofmt check and go vet
```

Equivalent direct commands:

```sh
go build -o bin/mello .
go test ./...
gofmt -l .
go vet ./...
```

## Project layout

| Path | Purpose |
|------|---------|
| `cmd/` | command implementations and argument parsing |
| `internal/mello/` | Mello API client and data types |
| `internal/config/` | credential and profile storage |
| `internal/sync/` | local working-copy and synchronization engine |
| `internal/ui/` | terminal output helpers |

## Coding conventions

- Code must be formatted with `gofmt` and pass `go vet`.
- Keep the dependency footprint at zero: prefer the standard library.
- Public functions and types carry doc comments; the first word is the
  identifier name.
- Add or update tests for any behavioral change. Synchronization logic in
  `internal/sync` is covered by table-style tests against an in-memory API
  stub; API request shaping in `internal/mello` is covered with `httptest`.

## Pull requests

1. Create a feature branch.
2. Make focused changes with descriptive commits.
3. Ensure `make lint` and `make test` pass.
4. Open a pull request describing the change and its motivation.

## Releasing

Releases are automated with [GoReleaser](https://goreleaser.com) and GitHub
Actions. To cut a release:

1. Update `CHANGELOG.md`, moving entries from *Unreleased* into a new version
   section.
2. Tag the commit and push the tag:

   ```sh
   git tag v1.0.0
   git push origin v1.0.0
   ```

The `release` workflow runs the test suite, cross-compiles binaries for Linux,
macOS, and Windows (amd64 and arm64), produces archives and a `checksums.txt`,
and publishes them to a GitHub release. The configuration lives in
`.goreleaser.yaml`.

To validate the release build locally without publishing:

```sh
goreleaser release --snapshot --clean
```

## Continuous integration

The `ci` workflow runs `go vet`, `go test`, and `go build` on Linux, macOS, and
Windows, and checks formatting with `gofmt`, on every push and pull request.

## Reporting issues

When filing a bug, please include the command you ran, the observed output
(with any token redacted), the expected behavior, and the output of
`mello version`.
