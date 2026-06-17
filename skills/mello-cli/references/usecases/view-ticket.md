# Use case: inspect a ticket in full

```sh
mello ticket view PROJ-12                    # status, members, labels, column, board,
                                             # dates, description, last comments,
                                             # attachments, last history
mello ticket view PROJ-12 --comments 20 --history 20
mello ticket view PROJ-12 --no-comments
mello ticket view PROJ-12 --json             # raw server object — best for parsing
```

- A code (`PROJ-12`) is resolved to the ticket id via the working board; an id
  works directly.
- `--comments N` / `--history N` control how many of the most recent entries
  show (default 5; `0` = all).
- `--json` returns the exact server payload (all fields, whatever their shape) —
  use it when you need to read values programmatically.
