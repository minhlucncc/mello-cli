# Use case: multiple boards in one workspace

A `.mello` workspace can hold several boards. One is the **working board**;
commands default to it.

```sh
mello use ROAD                   # work on ROAD
mello use OPS                    # switch the working board to OPS
mello clone OPS                  # alternatively: attach AND mirror all of OPS's tickets

mello status --all               # plan across every board in the workspace
mello pull --all                 # refresh every tracked board
mello ticket list -b OPS          # one-off: target a board without switching
```

- `mello use` registers a board and makes it current (no tickets downloaded).
- `mello clone` registers it and mirrors all its tickets.
- `-b <board>` targets a specific board for a single command; `--all` (on
  `status`/`pull`/`push`) spans every board.
