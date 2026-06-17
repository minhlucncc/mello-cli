# Use case: handling conflicts

A conflict is when a ticket changed on **both** the local and server sides since
the last sync. `mello push` always checks the server first; `mello status
--remote` shows them ahead of time.

```sh
mello status --remote            # ! = conflict, ↓ = remote-only change, ~ = local change
```

Resolve one of two ways:

```sh
# Option A — take remote, then re-apply your edits
mello pull                       # bring remote changes down
# re-edit the ticket, then:
mello push

# Option B — force local over remote
mello push --force
```

When a conflict is detected during pull, the server's version of the ticket is
written to `ticket.remote.json` inside that ticket's folder so you can compare.
