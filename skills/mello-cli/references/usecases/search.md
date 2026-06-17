# Use case: search tickets

```sh
mello search "billing"                       # current workspace (all workspaces if outside one)
mello search "login 500" -w <workspace-id>   # a specific workspace
mello search "billing" --json | jq length    # scriptable
mello search "billing" --json | jq -r '.[] | "\(.ticket_code)\t\(.title)"'
```

Search uses the server's full-text search over ticket title and description.
Inside a `.mello` workspace it scopes to that workspace; outside one it searches
across every workspace the token can access.
