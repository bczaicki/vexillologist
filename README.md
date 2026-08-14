# vexillologist

> *vexillologist (n.): one who studies flags.*

A terminal tool for viewing and managing [LaunchDarkly](https://launchdarkly.com) feature flags — an interactive TUI for humans, and headless subcommands with JSON output for agents and scripts.

## Install

### Homebrew (macOS)

```sh
brew install bczaicki/tap/vex
```

### Prebuilt binary (no Go required)

Grab the archive for your platform from the [releases page](https://github.com/bczaicki/vexillologist/releases), or:

```sh
# macOS (Apple Silicon) example — adjust os/arch: darwin/linux/windows, arm64/amd64
curl -sL https://github.com/bczaicki/vexillologist/releases/latest/download/vex_$(curl -s https://api.github.com/repos/bczaicki/vexillologist/releases/latest | grep -o '"tag_name": "v[^"]*' | cut -d'v' -f2)_darwin_arm64.tar.gz | tar xz vex
sudo mv vex /usr/local/bin/
vex --version
```

On macOS, Gatekeeper may quarantine the downloaded binary; clear it with `xattr -d com.apple.quarantine $(which vex)` if you see a "cannot be opened" warning.

### With Go

With Go 1.26+ installed (`brew install go`), install straight from GitHub:

```sh
go install github.com/bczaicki/vexillologist/cmd/vex@latest
```

That puts `vex` in `$(go env GOPATH)/bin` (usually `~/go/bin`) — make sure it's on your `PATH`:

```sh
export PATH="$HOME/go/bin:$PATH"   # add to ~/.zshrc
```

Or build from a clone:

```sh
git clone https://github.com/bczaicki/vexillologist.git
cd vexillologist
make install      # go install with a version stamp
# or: make build  # → bin/vex, without touching your PATH
```

### Quick start

```sh
export LD_API_TOKEN=api-xxxx                     # LaunchDarkly → Account settings → Authorization
vex projects list                                # verify auth, find your project key
vex config set project=my-project environment=staging
vex                                              # launch the TUI
```

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

The flag list is a **state matrix**: one row per flag, one column per environment, so you can see where a flag is on without switching environments.

```
  flag                          dev   test  staging  producti  status
  connect-1-asset-tags           ○     ○       ●        ●      active
  connect-1-branch-filter        ●     ●       ●        ●      launched
  connect-1-device-commands ⚑    ○     ○      [●]       ●      inactive
  only-in-dev                    ●     –       –        –      new
```

`●` on · `○` off · `–` the flag has no config in that environment at all, which is a different fact from off · `⚑` archived.

`←/→` moves the **column cursor** — the bracketed cell. That column *is* the current environment: it's what `t` toggles, what the status bar shows, and what the env-relative sorts and evaluation statuses are computed against. Promoting a flag along dev → test → staging → prod is arrow keys along one row rather than four environment switches, and each column keeps its own guardrail (toggling the `production` column still demands the typed env key).

The detail view is opened on demand with `enter` and replaces the matrix until you `esc` back, so scanning many flags and digging into one stay separate modes — but `t`/`space` toggles from either.

| Key | Action |
| --- | --- |
| `↑/↓` `j/k` | move between flags |
| `←/→` `h/l` | move the environment column (sets the current environment) |
| `/` | fuzzy filter |
| `enter` | open the detail view (loads full targeting; `↑/↓` scrolls it) |
| `esc` | close the detail view / clear the filter |
| `t` or `space` | toggle flag on/off in the cursor's environment |
| `s` | cycle sort: key → name → created → **longest-on** → recent → status |
| `n` | create a new flag |
| `a` | archive flag (all environments) |
| `u` | restore an archived flag |
| `A` | switch between active and archived flags (archived rows are marked `⚑`) |
| `E` / `P` | jump the column cursor to an environment / switch project |
| `r` | refresh |
| `q` | quit |
| `pgup/pgdn` | page through a long list |

The `status` column is the flag's LaunchDarkly **evaluation status** in the cursor's environment — `new`, `active`, `launched` (serving one variation to everyone — retirement candidate), or `inactive` (no evaluations for 7+ days — archive candidate). It's dropped on narrow terminals to keep the matrix readable. The `longest-on` sort puts on-flags first, longest-unchanged first, and the `status` sort floats archive candidates to the top; because both are defined relative to an environment, moving the column cursor re-sorts (keeping your place on the same flag).

The detail view shows each flag's camelCase key — the identifier the LaunchDarkly React SDK exposes via `useFlags()` — plus its creation age, per-environment change ages, and evaluation status.

**Guardrails**: environments that LaunchDarkly marks critical, are named `prod`/`production`, or are listed under `guarded_environments:` in the config file require typing the environment key to confirm a toggle. Archiving always requires typing the flag key.

## CLI (for agents and scripts)

Every operation is available non-interactively. Pass `--json` for machine-readable output on stdout.

```sh
vex projects list --json
vex envs list -p my-project --json
vex flags list -p my-project -e staging --json
vex flags list -p my-project -e staging --filter geofence --archived
vex flags list -p my-project -e staging --sort longest-on     # what's been on the longest
vex flags list -p my-project -e staging --sort status         # archive candidates first
vex flags statuses -p my-project -e staging --json            # evaluation status per flag
vex flags get my-flag-key -p my-project -e staging --json
vex flags toggle my-flag-key -p my-project -e staging --on --yes --json
vex flags create new-flag-key -p my-project --name "New Flag" --tags team-a --json
vex flags archive old-flag-key -p my-project --yes --json
vex flags restore old-flag-key -p my-project --yes --json
```

`--sort` accepts `key`, `name`, `created` (newest first), `longest-on` (on-flags, oldest change first), `recent` (latest change first), and `status` (inactive → launched → new → active). The human table includes `CHANGED` (time since the flag last changed in the environment) and `STATUS` columns; `--json` returns the raw flag resources in the sorted order, and `vex flags statuses` returns the status map for scripting.

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
