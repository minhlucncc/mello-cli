# Use case: first-time setup & authentication

Authenticate once with a Mello personal access token (`mello_pat_…`), then create
a workspace and pick a board.

```sh
mello auth login                 # paste your token (hidden input)
mello auth status                # verify identity + selected workspace
cd ~/my-project
mello init                       # create a .mello workspace here (needs nothing)
mello use ROAD                   # choose the board by code/slug (resolved across your workspaces)
```

Non-interactive / CI (no `auth login` needed):

```sh
export MELLO_TOKEN=mello_pat_xxx
export MELLO_WORKSPACE=<workspace-id>   # only if the token has several workspaces
mello board list --json
```

Notes:

- A token with one workspace auto-selects it; with several you are prompted, or
  pass `-w <id>`.
- `mello auth status` is the quickest "am I logged in?" check. "not logged in"
  ⇒ run `mello auth login` first.
