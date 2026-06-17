# Configuration

`mello` reads configuration from a file and from environment variables.
Environment variables take precedence, allowing the same installation to be used
both interactively and in automated environments.

## Configuration file

Credentials and preferences are stored in:

```
~/.config/mello/config.json
```

The file is created with mode `0600` and is keyed by named profile:

```json
{
  "current": "default",
  "profiles": {
    "default": {
      "base_url": "https://mello.mezon.vn/api/v1",
      "token": "mello_pat_…",
      "workspace_id": "…"
    }
  }
}
```

The directory can be relocated with the `MELLO_CONFIG_DIR` environment variable.

## Profiles

A profile is a named set of credentials and defaults. The active profile is used
unless overridden with `-p`/`--profile`.

```sh
mello auth login --profile staging --base-url https://staging.example/api/v1
mello board list --profile staging
```

`auth login` writes to (and activates) the selected profile; `auth logout`
removes only the token, leaving other settings intact.

## Environment variables

| Variable | Effect |
|----------|--------|
| `MELLO_TOKEN` | personal access token; overrides the stored token |
| `MELLO_BASE_URL` | API base URL; default `https://mello.mezon.vn/api/v1` |
| `MELLO_PROFILE` | active profile name |
| `MELLO_WORKSPACE` | default workspace id |
| `MELLO_CONFIG_DIR` | configuration directory; default `~/.config/mello` |
| `NO_COLOR` | disable colored output when set |

## Resolution order

For each setting, the first defined source wins:

1. Command-line flag (`--base-url`, `-p`, …).
2. Environment variable.
3. The active profile in the configuration file.
4. Built-in default.

## Security notes

- The token is transmitted as an HTTP `Authorization: Bearer` header and is never
  written to logs or terminal output (it is masked in `auth status`).
- The configuration file is written with owner-only permissions. When supplying
  the token through `--with-token` or environment variables in CI, prefer the
  platform's secret-management facilities over committing it to a file.
