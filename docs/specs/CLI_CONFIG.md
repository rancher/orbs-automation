# CLI Config

**Status:** implemented, except what [Not built yet](#not-built-yet) lists.

How a single invocation of `gero` gets its settings, and how it finds the central application config. Flags and the command tree are in [CLI_FRAMEWORK](CLI_FRAMEWORK.md). Inferring values from the working directory, and resolving repo paths, are in [WORKSPACE](WORKSPACE.md).

## Terms

Two kinds of config exist, and the docs keep them apart.

* **Application config** describes the world `gero` operates on: projects, Rancher lines and per-repo workflow settings. It is the multi-document YAML in `config/` and a target repo's `gero.yaml` ([APP_CONFIG](APP_CONFIG.md)). Versioned by `apiVersion`.
* **CLI config** describes how one invocation behaves: where to find the application config, which remote counts as upstream, output format, verbosity. It comes from flags, environment variables and an optional user file. It never holds application facts, and it never holds secrets.

Code mirrors the split: `internal/apis`, `internal/appconfig` and `internal/chain` for application config (the types, the provider and the joined model, behind `internal/workspace`), `internal/gero/clicfg` for CLI config ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)).

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
              internal/gero/clicfg  ->  Settings (resolved, immutable)
                                        |
                                  command RunE
```

`GH_TOKEN` and `GITHUB_TOKEN` keep their meaning and are consumed by the GitHub access layer ([GITHUB_ACCESS](GITHUB_ACCESS.md)), not by this loader. A few values are read from the working directory, such as the local path of a repository ([WORKSPACE](WORKSPACE.md)); the rest are never guessed from it.

### Flag source

`internal/gero/clicfg` does not import cobra ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)), so `cmd/` hands it a framework-neutral lookup, built once in `PersistentPreRunE` over `cmd.Flags()`, in the same style as `getenv`:

```go
type FlagLookup func(key string) (value string, set bool) // set == Flags().Changed
type Inputs struct {
    Flags        FlagLookup
    Getenv       func(string) string
    UserFile     io.Reader // nil when absent
    UserFileName string    // for error messages
    Home         string    // expands a leading ~ in paths
    Facts        Facts     // what only the process knows of its terminal
}
func Load(Inputs) (Settings, error)
```

Keys are table-driven (file key, derived env var, flag name, default, validator). The file is decoded strictly (`KnownFields(true)`), which is why `go.yaml.in/yaml/v4` is a direct dependency ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)). Global flags go through `clicfg`. Per-command flags stay in `cmd/` and reach `internal/cli/<cmd>` as an options struct.

## User file and keys

### The file

* Path: `$XDG_CONFIG_HOME/gero/config.yaml`, else `~/.config/gero/config.yaml`. `GERO_CONFIG` overrides the path. We do not use `os.UserConfigDir` because it points at `~/Library/Application Support` on macOS, and `~/.gero` was rejected to match `gh`'s layout.
* Optional. A missing file is fine; a malformed file, or a key with a value that is not allowed, is a loud error naming the file and key. A key gero does not know (a typo, or one from a newer gero) is not: it is ignored, the keys that are valid still apply, and every command prints `warning: <file>: line N: unknown key "x", ignored` on stderr. The schema still marks it, so an editor underlines it.
* `gero config show` lists the file it loaded as `userFile`, first in the table, with `env` as its source when `GERO_CONFIG` named it and `default` otherwise, or `(none)` when no file applies. `gero doctor` checks that the file loads ([DOCTOR](DOCTOR.md)).
* `gero init` writes a first file: `githubUser`, `fork`, `upstream`, `branchPrefix`, `centralConfigDir`, `repos.search` and `repos.cloneRoot`, those it is given or that are answered, after the schema comment. It will not replace a file that exists unless `--force` ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)).
* No secrets. Tokens stay with `gh` or the environment.
* **CI ignores it** unless `GERO_CONFIG` is set explicitly, so a runner never behaves differently because of a stray file. What counts as CI is decided in one place ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#detecting-the-context)).
* The main use is per-developer behavior that must not leak into the shared tool. The motivating case: locally `origin` is a developer's fork, while in CI `origin` is the real source, so `upstream` tells `gero` which remote to treat as the real source.
* It never holds a tool decision, only where things are and how output looks, so CI results stay reproducible. `team` says whose projects this person works for, not what the tool does with them, and the default is the same in CI, so a stray file changes nothing there. There is deliberately no layout key: it would encode a workstation convention not everyone follows. `repos.cloneRoot` is not one, since it says where a new clone goes and not how existing clones are arranged; the layout under it is fixed at `<root>/<name>`.

### Keys and environment

Names are not final **(leaning)**. A dot in a key nests it in the file, and becomes an underscore in the environment variable (`repl.prompt` is `GERO_REPL_PROMPT`).

| File key | Env var | Flag | Meaning |
|---|---|---|---|
| `centralConfigDir` | `GERO_CENTRAL_CONFIG_DIR` | `--central-config-dir` | The local clone of the central repo's `config/` directory, where the central application config is. Named for that, since gero's own config directory is the user's, where the user file is. Takes precedence over the release bundle ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)) unless `--config-source=bundle` is given |
| `configSource` | `GERO_CONFIG_SOURCE` | `--config-source` | `auto` (default), `bundle` or `dir`: where the central config comes from ([below](#locating-the-central-application-config)). It is a per-user preference and overlaps no other key, so it may live in the file; a developer sets `bundle` there and flips to `dir` for one run |
| `bundleChannel` | `GERO_BUNDLE_CHANNEL` | `--bundle-channel` | `auto` (default), `stable` or `prerelease`: which releases the central bundle may come from ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md#releases-fetching-and-verifying)). `auto` follows the running gero: a pre-release gero reads pre-release bundles, a stable one only stable ones |
| n/a | `GERO_BUNDLE_FILE` | `--bundle-file` | Path of one bundle file, downloaded by hand, to run against the config of an exact release. **Not a user-file key**: it is the explicit one-off, and a file that pinned it would fix every command to one release |
| `upstream` | `GERO_UPSTREAM` | `--upstream` | Remote or repo treated as the real source |
| `team` | `GERO_TEAM` | `--team` | The **active team context gero works in when run**, a lowercase slug, default `orbs`. A project whose `managedBy` is this team is **owned**: gero ships from it, `gero repo clone --all` clones it, and its `from: producer` steps run. `shared` is not a team and is refused. A project of another team is observed, and `gero ship` takes one as the producer only with `--to` naming a consumer the team owns or one that is `shared` ([PUSH_TO_REPO](PUSH_TO_REPO.md#shipping-another-teams-release)). The default means CI, which reads no user file, acts as ORBS unless `GERO_TEAM` says otherwise. A team that is the `managedBy` of no project owns nothing, so a typo shows as every project being observed |
| `output` | `GERO_OUTPUT` | `--output`, `--json` | `text` (default), `json` or `env` |
| `color` | `GERO_COLOR` | `--color`, `--no-color` | `auto` (default), `always` or `never`. `auto` defers to `NO_COLOR`, `CLICOLOR_FORCE`, `TERM` and TTY detection ([CLI_OUTPUT](CLI_OUTPUT.md#terminal-capabilities)) |
| (none) | `GERO_DRY_RUN` | `--dry-run` | `true` or `false` (default). **Not a user-file key**: a file that turned it on would silence every write |
| n/a | `GERO_NON_INTERACTIVE` | `--non-interactive`, `--yes` (on the write commands) | `true` skips the confirmation a write command asks before it starts ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)). No user-file key |
| n/a | `GERO_BOT_USER` | n/a | The bot's login in CI, for example `rancher-scc-bot[bot]`, supplied by the workflow with the token. Names the commit identity ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#the-actor)). Recorded as `Settings.BotUser`; no flag, no user-file key |
| n/a | `GERO_LOG` | `-v`, `-vv` | Log level, `debug`, `info`, `warn` (default) or `error` ([CLI_OUTPUT](CLI_OUTPUT.md)). A setting named `log`; no user-file key |
| `repl.prompt` | `GERO_REPL_PROMPT` | n/a | The prompt of `gero repl`, a template of plain text and placeholders (`{project}`, `{dir}`, `{path}`), default `gero{project: (%s)}> ` ([REPL](REPL.md#the-prompt)). No flag: a prompt is not a per-invocation choice. Checked by `gero repl` when it starts, and by no other command |
| n/a | `GERO_OFFLINE` | n/a | `true` makes `gero config fetch` fail and a command skip the network where it has a local way. The central config bundle is never fetched by another command, so offline changes nothing about which one is read. Env only: a file that turned it on would silence the network for every command |
| n/a | `GERO_CONFIG` | n/a | Path of the user file |
| n/a | `GERO_CONTEXT` | n/a | `ci` or `workstation`, forcing the execution context ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#detecting-the-context)). Recorded as `Settings.CI`; no user-file key |

Env and flag names derive from the file key by one rule, so the table is a check, not a source of truth. The loader is table-tested for precedence and for unknown keys ([TESTING](TESTING.md)).

File-only keys, all **(leaning)**:

| File key | Meaning |
|---|---|
| `fork` | Remote that holds the developer's fork |
| `pushTo` | Where `gero ship` pushes branches, `fork` or `upstream`. Without it, the fork when one is set, else the upstream ([PUSH_TO_REPO](PUSH_TO_REPO.md#where-the-branch-is-pushed)) |
| `githubUser` | Used to build fork refs for PRs |
| `branchPrefix` | Lead-in for branches `gero` creates (for example `dan/`). The key `{source}-{tag}-{target}` is always appended and cannot be set ([PUSH_TO_REPO](PUSH_TO_REPO.md#branches-and-pr-identity)) |
| `signCommits` | `true` makes git sign the commits `gero ship` makes, with the engineer's own key; env `GERO_SIGN_COMMITS` beats the file. A commit that cannot be signed fails the target, never falls back to unsigned ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#push-credentials-and-signing)) |
| `repos.paths` | Map of central-config repo slug (`spec.repo.name`, `owner/name`) to a local clone path |
| `repos.search` | Directories scanned for clones through any directories that group them, to a depth limit, matched to slugs by remote URL. No directory layout is assumed. An entry that is not a directory is skipped by the scan, so `gero doctor` fails on it and every other command names it only with `-v` |
| `repos.cloneRoot` | The one directory `gero repo clone` puts new clones in, as `<root>/<name>`, the repository's name. Precedence: `--into`, then env `GERO_REPOS_CLONE_ROOT`, then this key; with none set the command fails naming the three. It is scanned like a `repos.search` entry, so a clone is found again without another step. A `repos.paths` entry for the slug beats the root. Never inferred from `repos.search` |
| `repos.overrides.<slug>.upstream` | Per-repo upstream remote name |
| `repos.overrides.<slug>.pushTo` | Per-repo `pushTo`, which beats the project's and the user's own |

```yaml
# All keys optional. Unknown keys are warned about and ignored. No secrets.
# "~" and $ENV are expanded in paths; relative paths are an error.
centralConfigDir: ~/GitProjects/SUSE/orbs-automation/config   # a local clone, only for developing the config
configSource: bundle   # auto (default), bundle or dir
bundleChannel: auto    # auto (default), stable or prerelease
output: text
color: auto            # auto (default), always or never
upstream: upstream
team: orbs             # the active team context; default orbs
repl:
  prompt: "gero{project: (%s)}> "   # {project} {dir} {path}; {{ and }} are braces
fork: origin
githubUser: danpock
branchPrefix: dan/
pushTo: fork
repos:
  search:
    - ~/GitProjects/SUSE
  cloneRoot: ~/GitProjects/SUSE
  paths:
    rancher/rancher: ~/GitProjects/SUSE/rancher-src
  overrides:
    rancher/scc-operator:
      upstream: rancher
      pushTo: upstream
```

The keys and their allowed values are also published as a JSON Schema, `generated/jsonschema/user-config.schema.json`, for an editor ([APP_CONFIG](APP_CONFIG.md#json-schemas)).

## Locating the central application config

gero assumes the user has no clone of the central repo. The **bundle** of the central config ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md)), the one built into the binary or a newer one that `gero config fetch` cached, is the first layer, and a local directory is a **local development mode** on top of it, for someone changing the config itself.

`--config-source` (`GERO_CONFIG_SOURCE`, or `configSource` in the user file) says which:

* `auto`, the default: a local source if one is present, else the bundle. The local sources, in order: `--central-config-dir`, `GERO_CENTRAL_CONFIG_DIR`, `centralConfigDir` in the user file, then `./config` when the working directory is a checkout of the central repo (reported as source `cwd`). Having one set is what turns local development mode on.
* `bundle`: the bundle, always. A `centralConfigDir` that is set stays set and is ignored, so a developer can keep it and still read what everyone else reads.
* `dir`: the local sources only, never the bundle, so it never reaches the cache or the network. With none present it fails.

The flag beats the environment, which beats the file, so `configSource: bundle` in the file and `--config-source=dir` on one command reads the local directory for that run.

`--bundle-file <path>` (`GERO_BUNDLE_FILE`; never the user file) names one bundle file the user downloaded, to run against an exact release. It is the config source, whatever else is set: no fetch, no cache, no network. Its format and versions are checked as for any bundle; its signature and checksum are not, since the user chose the file, as with a local directory. Together with `--config-source=dir` it is an error.

Which releases the bundle may come from is `bundleChannel` ([CENTRAL_BUNDLE](CENTRAL_BUNDLE.md#releases-fetching-and-verifying)).

A source that is named but broken (a missing directory or file, invalid or empty config) is an error and never falls back to another, so a typo cannot run on published config. If none resolves, the command fails with a message listing these places and, when the bundle was tried, why it was not available. There is no runtime fetch of config from another branch ([APP_CONFIG](APP_CONFIG.md)). CI ignores the user file unless `GERO_CONFIG` names it, so a stray file cannot move a runner. A job that wants the config of a checkout gives `GERO_CENTRAL_CONFIG_DIR` or `--central-config-dir`, or a generated user file, pointing at the `config` directory of that checkout; with none it reads the bundle in the binary, the config of its own release. `gero config fetch` is the one command that looks for a newer bundle, and `GERO_OFFLINE=true` makes it fail; it is an environment variable and never a user-file key, so a file cannot turn the network off for every command.

## Not built yet

* A `host` setting (`GERO_HOST`, `--host`) for a GitHub host other than github.com, built only if someone asks. Today the host is `GH_HOST`, else the one in `gh`'s config, else github.com ([GITHUB_ACCESS](GITHUB_ACCESS.md)).
* A fork whose repository name differs from the upstream's, under `githubUser`.

## Open questions

* Final names for the user-file keys.
