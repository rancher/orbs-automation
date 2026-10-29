# CLI Config

**Status:** agreed. Items marked **(leaning)** are proposals. Built: `internal/clicfg` (flags, env, user file, defaults, source tracking) and the global flags in `cmd/`. `gero config show` (resolved values and their sources, as text, json or env) and central config directory resolution (section 4, `internal/cli/configdir`, shown as source `cwd` for the `./config` fallback). Not built: CWD inference (`internal/workspace`), the `inferred` source, and any other command that reads `Settings`.

How a single invocation of `gero` gets its settings, and how it finds the central application config. Flags and the command tree are in [CLI_FRAMEWORK](CLI_FRAMEWORK.md).

## 1. Terms

Two kinds of config exist, and the docs keep them apart.

* **Application config** describes the world `gero` operates on: projects, Rancher lines, external projects, and per-repo workflow settings. It is the multi-document YAML in `config/` and a target repo's `gero.yaml` (IMPLEMENTATION §7). Versioned by `apiVersion`.
* **CLI config** describes how one invocation behaves: where to find the application config, which remote counts as upstream, output format, verbosity, host. It comes from flags, environment variables and an optional user file (section 2). It never holds application facts, and it never holds secrets.

Code mirrors the split: `internal/config` for application config, `internal/clicfg` for CLI config ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)).

## 2. Sources and precedence

Highest wins:

1. **Flags**, detected by whether the flag was explicitly set (cobra's `Flags().Changed()`, read in `cmd/`) so a flag's default never masks a lower source. `internal/clicfg` does not import cobra ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 3), so `cmd/` passes it a framework-neutral flag source: the values, and which were set.
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

`GH_TOKEN` / `GITHUB_TOKEN` keep their meaning and are consumed by the GitHub access layer (IMPLEMENTATION §3), not by this loader.

## 3. User file and keys

### User file

* Path: `$XDG_CONFIG_HOME/gero/config.yaml`, else `~/.config/gero/config.yaml`. `GERO_CONFIG` overrides the path. We do not use `os.UserConfigDir` because it points at `~/Library/Application Support` on macOS. `~/.gero` was considered and rejected to match `gh`'s layout.
* Optional. A missing file is fine; a malformed file, or one with an unknown key, is a loud error naming the file and key.
* No secrets. Tokens stay with `gh` or the environment.
* **CI ignores it** unless `GERO_CONFIG` is set explicitly, so a runner never behaves differently because of a stray file.
* The main use is per-developer behavior that must not leak into the shared tool. The motivating case: locally `origin` is a developer's fork, while in CI `origin` is the real source, so `upstream` tells `gero` which remote to treat as the real source.

### Keys and environment

Candidate set, names not final **(leaning)**:

| File key | Env var | Flag | Meaning |
|---|---|---|---|
| `configDir` | `GERO_CONFIG_DIR` | `--config-dir` | Where the central application config is (section 4) |
| `upstream` | `GERO_UPSTREAM` | `--upstream` | Remote or repo treated as the real source |
| `output` | `GERO_OUTPUT` | `--output`, `--json` | `text` (default), `json` or `env` |
| `host` | `GERO_HOST` | `--host` | GitHub host, if we ever target non-github.com (IMPLEMENTATION §3) |
| n/a | `GERO_LOG` | `-v` | Log level ([CLI_OUTPUT](CLI_OUTPUT.md) section 2) |
| n/a | `GERO_CONFIG` | n/a | Path of the user file |

Env and flag names derive from the file key by one rule, so the table is a check, not a source of truth. The loader is table-tested for precedence and for unknown keys ([TESTING](TESTING.md)).

File-only keys (no flag or env var), all **(leaning)**:

| File key | Meaning |
|---|---|
| `fork` | Remote that holds the developer's fork; where `gero` pushes branches |
| `githubUser` | Used to build fork refs for PRs |
| `branchPrefix` | Prefix for branches `gero` creates (for example `dan/`) |
| `repos.paths` | Map of central-config repo slug (`spec.repo.name`, `owner/name`) to a local clone path |
| `repos.search` | Directories scanned to depth 2 for clones, matched to slugs by remote URL. No directory layout is assumed |
| `repos.overrides.<slug>.upstream` | Per-repo upstream remote name |

Example:

```yaml
# All keys optional. Unknown keys are an error. No secrets.
# "~" and $ENV are expanded in paths; relative paths are an error.
configDir: ~/GitProjects/SUSE/orbs-automation/config
output: text
upstream: upstream
fork: origin
githubUser: danpock
branchPrefix: dan/
repos:
  search:
    - ~/GitProjects/SUSE
  paths:
    rancher/rancher: ~/GitProjects/SUSE/rancher-src
  overrides:
    rancher/scc-operator:
      upstream: rancher
```

The user file never holds a tool decision, only where things are and how output looks, so CI results stay reproducible. There is deliberately no `root` or layout key: it would encode a workstation convention not everyone follows.

## 3a. Inferred defaults

Most keys can be left unset because they are inferred from the repo at the working directory (found with `repo.Discover`), its remotes and its branch. Full precedence per value:

```
flag > env > user file > inferred from CWD repo > built-in default > error
```

| Value | Inferred default |
|---|---|
| Current repo slug | `owner/name` parsed from remote URLs, matched against central-config `spec.repo.name` |
| Local path for a slug | The CWD repo if its slug matches; else `repos.paths`; else the `repos.search` scan; else the flat sibling `<parent of CWD repo>/<name>` |
| `configDir` | Nearest ancestor of the CWD containing `config/*.project.yaml` |
| `upstream` | Remote whose URL matches the slug; else the only remote; in CI, `origin` |
| `fork` | Remote with the same repo name under a different owner |
| `githubUser` | Owner of the fork remote's URL (no network call) |
| Release line | `main` is the development line; `release/vX.Y` is that line |
| `output` | `text`; color follows `NO_COLOR` and TTY detection |
| `branchPrefix` | None |

Rules:

* Inference never picks silently among several candidates. Ambiguity is an error naming the key to set.
* It is lazy and per command: a command that needs no remote does not fail because the CWD is not a repo.
* A user-file value always beats inference.
* `gero config show` reports each value's source (`flag`, `env`, `file`, `inferred (reason)`, `default`). Verbose mode logs inferred values before any write (push, PR).
* `internal/clicfg` stays pure (flags, env, file, defaults; unset stays zero) and does not import git. A separate resolver package (working name `internal/workspace`) takes `Settings` and a small repo-access interface and does the inference.

## 4. Locating the central application config

Commands that read the central `config/` find it in this order:

1. `--config-dir`
2. `GERO_CONFIG_DIR`
3. `configDir` in the user file
4. `./config`, when the working directory is a checkout of the central repo

If none resolves, the command fails with a message listing these four places. There is no runtime fetch of config from another branch (IMPLEMENTATION §7, "Avoid"). In CI the central repo is checked out and `GERO_CONFIG_DIR` or `--config-dir` is set explicitly in the workflow.

## Open questions

* How commands receive resolved flag values. Decided: global flags go through `internal/clicfg`; per-command flags stay in `cmd/` and reach `internal/cli/<cmd>` as an options struct.
* Final names for the user-file keys.

## Flag source

`cmd/` hands `internal/clicfg` a framework-neutral lookup, built once in `PersistentPreRunE` over `cmd.Flags()`, in the same style as `getenv`:

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

Keys are table-driven (file key, derived env var, flag name, default, validator). The file is decoded strictly (`KnownFields(true)`), which makes `go.yaml.in/yaml/v3` a direct dependency ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)).
