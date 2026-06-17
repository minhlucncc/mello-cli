# Use case: find tickets assigned to me

`me`/`--mine` resolve to the authenticated user. Tickets can have multiple
members; `--mine` matches if you are one of them.

```sh
mello ticket list --mine                       # on the working board
mello ticket list --mine --status open
mello ticket list --mine --column "In Progress"
mello ticket list --assignee me --json | jq -r '.[].ticket_code'
```

Other filters (combine freely):

```sh
mello ticket list --column Todo                # everyone's, in one column
mello ticket list --status in_progress
mello ticket list --assignee <user-id>         # someone specific
mello ticket list -b OPS --mine                # a different board without switching
```

The list shows: ticket code, column, members (first member `+N`), status, title.
