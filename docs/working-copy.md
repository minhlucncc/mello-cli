# The local working copy

`mello sync clone` creates a `.mello` directory that mirrors a Mello board on
local disk. You edit the mirrored files offline, review the resulting changes,
and synchronize them with the server on demand.

## Directory layout

```
.mello/
  state.json                       # tracked board, sync cursor, and per-ticket baselines
  journal.log                      # an audit line per push
  boards/<board>/
    tickets/<ticket>/
      ticket.md                    # editable: YAML front matter + description body
      ticket.json                  # server baseline used for diffing (do not edit)
      ticket.remote.json           # written only on conflict: the server's version
      comments/
        0001-<author>.md           # a synchronized comment (has an `id`)
        <draft>.md                 # a locally authored comment (no `id`)
      comments.json                # comment baseline
      attachments/
        <file>                     # attachment files
      attachments.json             # attachment baseline
```

Commands run from anywhere inside the working copy; the `.mello` directory is
located by searching parent directories.

## Three reference points

Each ticket is tracked at three points, analogous to a version-controlled file:

- **Server** — the authoritative state, retrieved through the API.
- **Baseline** — the server state captured at the last synchronization, stored in
  `ticket.json` and summarized by a content hash in `state.json`.
- **Working copy** — your local edits in `ticket.md` and the `comments/` and
  `attachments/` directories.

A change is determined by comparing content hashes, not timestamps, so only
genuine differences are sent.

## The ticket file

`ticket.md` contains editable front matter followed by the description:

```markdown
---
ticket: ROAD-1
id: 8f3c…
title: Draft the proposal
status: in_progress
assignee: user-123
column: In Progress
labels: [sales, q3]
---

Long-form description in Markdown.
```

The `ticket` and `id` fields are informational and ignored on push. Editing
`title`, `status`, `assignee`, `labels`, the `column`, or the body produces a
corresponding update. Changing `column` moves the ticket.

## The plan

`mello sync status` prints the set of pending changes:

| Marker | Meaning | Action on push |
|--------|---------|----------------|
| `+` | a ticket directory without a server id | create the ticket |
| `~` | modified fields, body, column, comments, or attachments | update, move, comment, or upload |
| `-` | a tracked ticket whose directory was removed | delete the ticket |
| `↓` | only the server changed (shown with `--remote`) | run `sync pull` |
| `!` | changed on both sides since the baseline | held back unless `--force` |

## Operations

- **Create** — add a directory containing a `ticket.md` (or run `mello new
  ticket`). On push the ticket is created and its server id is recorded.
- **Modify** — edit `ticket.md`. The minimal set of changed fields is sent.
- **Move** — change the `column` field to another column's name.
- **Comment** — add a Markdown file to `comments/`. Files without an `id` are
  posted on push and then replaced by the synchronized copy, so they are not sent
  again.
- **Attach** — place a file in `attachments/`. Files whose content hash is not in
  the baseline are uploaded.
- **Delete** — remove a ticket directory. On push the ticket is deleted on the
  server.

## Conflicts

`mello sync push` checks the server before applying changes. If a ticket changed
on both the local and server sides since the baseline, it is reported as a
conflict and skipped; the server's version is written to `ticket.remote.json` for
comparison. Resolve the conflict by editing `ticket.md`, or pass `--force` to push
the local version. `mello sync pull` likewise preserves local edits and records
conflicts rather than overwriting them.

## Versioning and auditing

`state.json` holds a `serial` that increments on each successful synchronization.
Every push appends a line to `journal.log` recording the serial, a timestamp, and
the number of creates, updates, deletes, and conflicts. These provide a simple,
inspectable history of synchronization activity.

## Incremental synchronization

`mello sync pull` requests only tickets modified since the last run, using a
cursor stored in `state.json`. Tickets deleted on the server are removed from the
working copy on the next pull.
