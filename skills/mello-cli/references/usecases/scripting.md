# Use case: scripting with JSON

Every read command supports `--json`. Prefer it whenever you need to read values
programmatically instead of scraping the table output.

```sh
mello ticket list --json | jq -r '.[] | "\(.ticket_code)\t\(.title)"'
mello ticket list --mine --json | jq length
mello ticket view PROJ-12 --json | jq '.ticket'          # raw server object
mello board list --json | jq -r '.[].board.code'
mello search "billing" --json | jq -r '.[].ticket_code'
```

Tips:

- `mello ticket view <id> --json` returns the **raw** server payload for a single
  ticket — the most complete source of its fields.
- Check exit codes: `0` success, `1` runtime error (message on stderr), `2`
  invalid usage.
- Run headless with `MELLO_TOKEN` (and `MELLO_WORKSPACE` if needed); no
  interactive login required.
- For always-fresh metadata set `MELLO_NO_CACHE=1`, or run `mello cache clear`.
