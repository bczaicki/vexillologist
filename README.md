# vexillologist

> *vexillologist (n.): one who studies flags.*

A terminal tool for viewing and managing [LaunchDarkly](https://launchdarkly.com) feature flags — an interactive TUI for humans, and headless subcommands with JSON output for agents and scripts.

## Install

```sh
make build        # → bin/vex
```

Requires Go 1.26+.

## Authentication

`vex` needs a LaunchDarkly **API access token** (Account settings → Authorization). Resolution order:

1. `--api-token` flag
2. `LD_API_TOKEN` env var (preferred)
3. `LD_ACCESS_TOKEN` (ldcli compatibility), then `LAUNCHDARKLY_API_TOKEN`
4. `token` in the config file (discouraged; a warning is printed when set)

Default project and environment live in `~/.config/vexillologist/config.yaml`:

```sh
vex config set project=my-project environment=staging
```

Per-invocation, `-p/--project` and `-e/--env` flags and the `LD_PROJECT`/`LD_ENVIRONMENT` env vars override the config file.

## TUI

```sh
vex               # or: vex tui
```

If no project/environment is configured you'll get a picker; otherwise you land straight on the flag list.

| Key | Action |
| --- | --- |
| `↑/↓` `j/k` | navigate |
| `/` | fuzzy filter |
| `enter` | load full targeting detail |
| `t` | toggle flag on/off in the current environment |
| `n` | create a new flag |
| `a` | archive flag (all environments) |
| `E` / `P` | switch environment / project |
| `r` | refresh |
| `q` | quit |

The detail pane shows each flag's camelCase key — the identifier the LaunchDarkly React SDK exposes via `useFlags()`.

**Guardrails**: environments that LaunchDarkly marks critical, are named `prod`/`production`, or are listed under `guarded_environments:` in the config file require typing the environment key to confirm a toggle. Archiving always requires typing the flag key.

## CLI (for agents and scripts)

Every operation is available non-interactively. Pass `--json` for machine-readable output on stdout.

```sh
vex projects list --json
vex envs list -p my-project --json
vex flags list -p my-project -e staging --json
vex flags list -p my-project -e staging --filter geofence --archived
vex flags get my-flag-key -p my-project -e staging --json
vex flags toggle my-flag-key -p my-project -e staging --on --yes --json
vex flags create new-flag-key -p my-project --name "New Flag" --tags team-a --json
vex flags archive old-flag-key -p my-project --yes --json
```

### Confirmations

Mutations prompt on a TTY. Headless (no TTY), `--yes` is **required** or the command exits with code 3. `--dry-run` prints the exact request payload without sending anything:

```sh
vex flags toggle my-flag -p proj -e production --off --dry-run --json
```

### Output envelope

Mutations emit:

```json
{"ok": true, "action": "turnFlagOn", "environment": "staging", "critical": false, "flag": {...}}
```

Errors (with `--json`) emit on stdout:

```json
{"ok": false, "error": {"code": "not_found", "message": "..."}}
```

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | success |
| 1 | LaunchDarkly API or unexpected error |
| 2 | usage/config error (missing token, project, or environment) |
| 3 | confirmation required but not given (add `--yes`) |
| 4 | resource not found |

## Development

```sh
make test    # unit tests (httptest fixtures, CLI, teatest TUI smoke tests)
make lint    # golangci-lint
make build   # version-stamped binary at bin/vex
```

Layout: `internal/ld` is the only package that talks HTTP (a minimal REST client pinned to LD API version `20240415`, using [semantic patches](https://launchdarkly.com/docs/api/feature-flags/patch-feature-flag) for toggle/archive). `internal/cli` (cobra) and `internal/tui` (bubbletea) are thin frontends over it, sharing the `ld.API` interface so tests run against an in-memory fake.
