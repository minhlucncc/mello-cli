# Use case: first-time setup & authentication

The CLI talks to one of two backends, auto-detected from the token:

- **`mello_pat_…`** personal access token → public `/api/v1` (official, limited).
- **session JWT / refresh token** from the web app → internal `/api` (full data:
  members, attachments, comments, history). Use this for complete data.

## Session auth (recommended — full data)

Get a token from the browser at mello.mezon.vn (DevTools, F12):

```sh
# Best: refresh token (auto-renews, ~30 days).
# Application → Cookies → mello.mezon.vn → refresh_token
mello auth login --refresh-token <refresh_token>

# Or: access token (~1h, no auto-renew).
# Application → Local Storage → mello.mezon.vn → mello.access_token
mello auth login --token <jwt>
```

With `--refresh-token` the CLI renews the access token automatically. Refresh
tokens are single-use/rotating — if it fails as expired, grab the current
`refresh_token` cookie again (and avoid sharing one session between CLI and
browser simultaneously).

## API key auth (public API)

```sh
mello auth login --token mello_pat_xxx
```

## Then set up the workspace + board

```sh
mello auth status                # verify identity + selected workspace
cd ~/my-project
mello init                       # create a .mello workspace here (needs nothing)
mello use PRESALES               # choose the board by code/slug
```

## Headless / CI

```sh
export MELLO_TOKEN=<token>
export MELLO_BASE_URL=https://mello.mezon.vn/api   # internal API (omit for public /api/v1)
export MELLO_WORKSPACE=<workspace-id>              # if the token has several
mello board list --json
```

`mello auth status` is the quickest "am I logged in?" check.
