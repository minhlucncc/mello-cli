# Use case: create a ticket

```sh
mello ticket create -t "Write the proposal"            # working board, first column
mello ticket create -t "Bug: login 500" -c "Triage" -d "Steps to reproduce…"
mello ticket create -t "Spec" --body-file ./spec.md     # description from a file
mello ticket create -t "Ops task" -b OPS -c "Backlog"   # a specific board/column
mello ticket create -t "X" --json                       # print the created ticket as JSON
```

- `-t/--title` is required.
- `-c/--column` is a column **name** (default: the board's first column).
- `-b/--board` defaults to the working board.

This creates the ticket **directly on the server**. To instead draft tickets
locally and push them in a batch, see `offline-work-loop.md` (`mello new ticket`).
