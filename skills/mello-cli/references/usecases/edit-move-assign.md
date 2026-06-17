# Use case: edit, move, and assign a ticket

Edit only the fields you pass:

```sh
mello ticket edit PROJ-12 --status in_progress --title "New title"
mello ticket edit PROJ-12 --assignee me                 # assign yourself
mello ticket edit PROJ-12 --labels bug,p1               # replaces labels
mello ticket edit PROJ-12 --body-file ./new-desc.md
```

Move across columns (note: `move` takes a column **id**, not a name):

```sh
mello column list                                       # find column ids
mello ticket move PROJ-12 --column <column-id> [--position 2]
```

Delete on the server (distinct from `untrack`, which is local-only):

```sh
mello ticket delete PROJ-12 -y
```

`ticket edit` and `ticket delete` are optional endpoints; if the instance does
not implement them the command prints a clear "not supported" message and exits
non-zero — report it rather than retrying.
