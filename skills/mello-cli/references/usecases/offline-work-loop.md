# Use case: the offline work-loop (local working set)

Use this when editing several tickets, working offline, or pushing a precise set
of changes. The **working set** is the local subset of tickets you have pulled or
created; `status`/`push` act only on it.

```sh
mello use ROAD                   # set the working board (downloads no tickets)
mello ticket list                # browse the remote set
mello pull PROJ-12 PROJ-15        # pull specific tickets into the working set
mello new ticket -t "New task" --column Todo   # or draft one locally

# edit files under .mello/boards/<board>/tickets/<ticket>/
#   ticket.md  → front matter (title, assignee, labels, column) + body (description)
#   comments/  → drop a Markdown file to post a new comment on push
#   attachments/ → drop a file to upload on push

mello status                     # review: + create, ~ update/move/comment/attach
mello push --dry-run             # preview what would be sent
mello push -m "Updated per review"   # apply; -m/--comment posts a note on each changed ticket
mello pull                       # later: refresh the tickets in the working set
mello untrack PROJ-12             # done → drop from the working set (stays on the server)
```

On push: field/body edits → `PATCH`; a changed `column` → move; new files in
`comments/` → posted; new files in `attachments/` → uploaded; `-m/--comment`
adds an extra comment announcing the change.

- `mello pull --all` mirrors **every** ticket on the board (full local copy).
- `untrack` is local-only; to delete on the server use `mello ticket delete`.
- See `conflicts.md` for handling edits made on both sides.
