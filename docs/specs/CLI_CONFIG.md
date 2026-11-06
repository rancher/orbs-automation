# CLI Config

**Status:** implemented for the flags, environment, user file and defaults (`internal/clicfg`) and for locating the central config. The file-only keys other than `repos.*` have no reader yet.

How a single invocation of `gero` gets its settings, and how it finds the central application config. Flags and the command tree are in [CLI_FRAMEWORK](CLI_FRAMEWORK.md). Inferring values from the working directory, and resolving repo paths, are in [WORKSPACE](WORKSPACE.md).

## Terms

Two kinds of config exist, and the docs keep them apart.

* **Application config** describes the world `gero` operates on: projects, Rancher lines, external projects, and per-repo workflow settings. It is the multi-document YAML in `config/` and a target repo's `gero.yaml` ([APP_CONFIG](APP_CONFIG.md)). Versioned by `apiVersion`.
* **CLI config** describes how one invocation behaves: where to find the application config, which remote counts as upstream, output format, verbosity, host. It comes from flags, environment variables and an optional user file. It never holds application facts, and it never holds secrets.

Code mirrors the split: `internal/config` for application config (types, with `internal/config/loader` behind `internal/workspace`), `internal/clicfg` for CLI config ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)).

## Sources and precedence

Highest wins:

1. **Flags**, detected by whether the flag was explicitly set (cobra's `Flags().Changed()`, read in `cmd/`) so a flag's default never masks a lower source.
2. **Environment**, `GERO_*`.
3. **User file.**
4. **Built-in defaults.**

```
 flags  >  GERO_* env  >  user file  >  defaults
                 \           |           /
                  v          v          v
              internal/clicfg  ->  Settings (resolved, immutable)
                                        |
                                  command RunE
```

`GH_TOKEN` and `GITHUB_TOKEN` keep their meaning and are consumed by the GitHub access layer ([GITHUB_ACCESS](GITHUB_ACCESS.md)), not by this loader. Values that are left unset can be inferred from the repository at the working directory, which sits between the user file and the defaults ([WORKSPACE](WORKSPACE.md)).

### Flag source

`internal/clicfg` does not import cobra ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)), so `cmd/` hands it a framework-neutral lookup, built once in `PersistentPreRunE` over `cmd.Flags()`, in the same style as `getenv`:

```go
type FlagLookup func(key string) (value string, set bool) // set == Flags().Changed
type Inputs struct {
    Flags    FlagLookup
    Getenv   func(string) string
    UserFile io.Reader // nil when absent
    CI       bool
}
func Load(Inputs) (Settings, error)
```

Keys are table-driven (file key, derived env var, flag name, default, validator). The file is decoded strictly (`KnownFields(true)`), which is why `go.yaml.in/yaml/v3` is a direct dependency ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)). Global flags go through `clicfg`. Per-command flags stay in `cmd/` and reach `internal/cli/<cmd>` as an options struct.

## User file and keys

### The file

* Path: `$XDG_CONFIG_HOME/gero/config.yaml`, else `~/.config/gero/config.yaml`. `GERO_CONFIG` overrides the path. We do not use `os.UserConfigDir` because it points at `~/Library/Application Support` on macOS, and `~/.gero` was rejected to match `gh`'s layout.
* Optional. A missing file is fine; a malformed file, or one with an unknown key, is a loud error naming the file and key.
* `gero config show` lists the file it loaded as `userFile`, first in the table, with `env` as its source when `GERO_CONFIG` named it and `default` otherwise, or `(none)` when no file applies. `gero doctor` checks that the file loads ([DOCTOR](DOCTOR.md)).
* No secrets. Tokens stay with `gh` or the environment.
* **CI ignores it** unless `GERO_CONFIG` is set explicitly, so a runner never behaves differently because of a stray file. What counts as CI is decided in one place ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#detecting-the-context)).
* The main use is per-developer behavior that must not leak into the shared tool. The motivating case: locally `origin` is a developer's fork, while in CI `origin` is the real source, so `upstream` tells `gero` which remote to treat as the real source.
* It never holds a tool decision, only where things are and how output looks, so CI results stay reproducible. There is deliberately no `root` or layout key: it would encode a workstation convention not everyone follows.

### Keys and environment

Names are not final **(leaning)**.

| File key | Env var | Flag | Meaning |
|---|---|---|---|
| `configDir` | `GERO_CONFIG_DIR` | `--config-dir` | Where the central application config is |
| `upstream` | `GERO_UPSTREAM` | `--upstream` | Remote or repo treated as the real source |
| `output` | `GERO_OUTPUT` | `--output`, `--json` | `text` (default), `json` or `env` |
| `color` | `GERO_COLOR` | `--color`, `--no-color` | `auto` (default), `always` or `never`. `auto` defers to `NO_COLOR`, `CLICOLOR_FORCE`, `TERM` and TTY detection ([CLI_OUTPUT](CLI_OUTPUT.md#terminal-capabilities)) |
| (none) | `GERO_DRY_RUN` | `--dry-run` | `true` or `false` (default). **Not a user-file key**: a file that turned it on would silence every write |
| n/a | `GERO_NON_INTERACTIVE` | `--non-interactive`, `--yes` | `true` skips the confirmation a write command asks before it starts ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)). No user-file key |
| n/a | `GERO_BOT_USER` | n/a | The bot's login in CI, for example `rancher-scc-bot[bot]`, supplied by the workflow with the token. Names the commit identity ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#the-actor)). Recorded as `Settings.BotUser`; no flag, no user-file key |
| `host` | `GERO_HOST` | `--host` | GitHub host, if we ever target non-github.com |
| n/a | `GERO_LOG` | `-v`, `-vv` | Log level, `debug`, `info`, `warn` (default) or `error` ([CLI_OUTPUT](CLI_OUTPUT.md)). A setting named `log`; no user-file key |
| n/a | `GERO_CONFIG` | n/a | Path of the user file |
| n/a | `GERO_CONTEXT` | n/a | `ci` or `workstation`, forcing the execution context ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#detecting-the-context)). Recorded as `Settings.CI`; no user-file key |

Env and flag names derive from the file key by one rule, so the table is a check, not a source of truth. The loader is table-tested for precedence and for unknown keys ([TESTING](TESTING.md)).

File-only keys, all **(leaning)**:

| File key | Meaning |
|---|---|
| `fork` | Remote that holds the developer's fork |
| `pushTo` | Where `gero ship` pushes branches, `fork` or `upstream`. Without it, the fork when one is set, else the upstream ([PUSH_TO_REPO](PUSH_TO_REPO.md#where-the-branch-is-pushed)) |
| `githubUser` | Used to build fork refs for PRs |
| `branchPrefix` | Prefix for branches `gero` creates (for example `dan/`) |
| `signCommits` | `true` makes git sign the commits `gero ship` makes, with the engineer's own key; env `GERO_SIGN_COMMITS` beats the file. A commit that cannot be signed fails the target, never falls back to unsigned ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#push-credentials-and-signing)) |
| `repos.paths` | Map of central-config repo slug (`spec.repo.name`, `owner/name`) to a local clone path |
| `repos.search` | Directories scanned to depth 2 for clones, matched to slugs by remote URL. No directory layout is assumed |
| `repos.overrides.<slug>.upstream` | Per-repo upstream remote name |
| `repos.overrides.<slug>.pushTo` | Per-repo `pushTo`, which beats the project's and the user's own |

```yaml
# All keys optional. Unknown keys are an error. No secrets.
# "~" and $ENV are expanded in paths; relative paths are an error.
configDir: ~/GitProjects/SUSE/orbs-automation/config
output: text
upstream: upstream
fork: origin
githubUser: danpock
branchPrefix: dan/
pushTo: fork
repos:
  search:
    - ~/GitProjects/SUSE
  paths:
    rancher/rancher: ~/GitProjects/SUSE/rancher-src
  overrides:
    rancher/scc-operator:
      upstream: rancher
      pushTo: upstream
```

## Locating the central application config

Commands that read the central `config/` find it in this order:

1. `--config-dir`
2. `GERO_CONFIG_DIR`
3. `configDir` in the user file
4. `./config`, when the working directory is a checkout of the central repo (reported as source `cwd`)

If none resolves, the command fails with a message listing these four places. There is no runtime fetch of config from another branch ([APP_CONFIG](APP_CONFIG.md)). In CI the central repo is checked out and `GERO_CONFIG_DIR` or `--config-dir` is set explicitly in the workflow.

## Not built yet

* The `inferred` source in `gero config show` ([WORKSPACE](WORKSPACE.md)).
* A reader for `fork`, `githubUser` and `branchPrefix`.

## Open questions

* Final names for the user-file keys.
