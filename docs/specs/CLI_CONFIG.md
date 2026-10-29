# CLI Config

**Status:** agreed. Items marked **(leaning)** are proposals. Built: `internal/clicfg` (flags, env, user file, defaults, source tracking) and the global flags in `cmd/`. `gero config show` (resolved values and their sources, as text, json or env) and central config directory resolution (section 4, `internal/cli/configdir`, shown as source `cwd` for the `./config` fallback). Not built: CWD inference and repo path resolution (`internal/workspace`, `gero config resolve repos`, section 3b), the `inferred` source, and any other command that reads `Settings`.

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
* `internal/clicfg` stays pure (flags, env, file, defaults; unset stays zero) and does not import git. A separate resolver package, `internal/workspace`, takes `Settings` and does the inference.
* **One door.** `workspace` is the only way a command gets "settings plus the world": the central config directory (section 4), the loaded application config, the current repo, and the local path, upstream and fork for a slug. Each rule exists once there, so the same CLI inputs cannot resolve differently in different commands. `cmd/` knows nothing about it. Each `internal/cli/<cmd>` calls `workspace.Open(settings, cwd, getenv)`, which takes plain values and builds its own real dependencies (file system, go-git opener). The command's logic takes the `Workspace` through a small interface it defines, so tests inject a fake. Resolution is lazy: `Open` does no I/O, and a method does only the work its caller needs.
* A command package may not locate the central config, discover a repo, or scan for clones itself. Lint enforces this ([PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 6).
* `configdir` (section 4) is the first such rule. It moves into `workspace` when `internal/config` gets its first consumer, and `gero config show` and `gero config resolve repos` then read through the same `Workspace` as every other command.
* `gero config show` may report one slug's resolved path with an `inferred (reason)` source, but it never enumerates repos. For an unset-from-file `repos.search` it shows a hint pointing at `gero config resolve repos`.

## 3b. Resolving repo paths

`gero config resolve repos [slug]` answers "where does each repo live on this machine, and how did `gero` decide?". `config show` reports raw values and their sources. This command runs the same `internal/workspace` resolver the other commands use, so its output is what they would act on. It is read-only and supports `--json`.

Resolution order for one slug is the one in section 3a: the CWD repo if its slug matches, then `repos.paths`, then the `repos.search` scan, then the flat sibling.

**The scan.** Each `repos.search` directory is scanned to depth 2 for clones. A directory is a clone if it contains `.git`. Its remote URLs are read from the repository config through `internal/git/repo` (go-git, no `git` subprocess) and parsed to `owner/name` slugs. Names of directories are never used for matching, so no layout is assumed. A clone whose upstream remote matches the slug is a strong match. A clone that only has a fork of the slug (same name, different owner) is a weak match and counts only when no strong match exists. Several matches of the same strength are ambiguous, never picked silently.

**Cost.** Most commands resolve one slug, usually from the CWD repo or `repos.paths`, and never scan. The scan runs only when those miss, and it is built once per command and matched against every slug asked for. Resolving all slugs is this command's case.

**Output.** One row per central-config slug (`spec.repo.name`) with the path and how it was found: `cwd`, `repos.paths`, `repos.search scan`, or `sibling`. Unresolved and ambiguous slugs are rows, not errors: an ambiguous row lists the candidate paths and names `repos.paths.<slug>` as the key to set. The command exits non-zero if any row is unresolved or ambiguous. An optional `<slug>` argument limits the output to that repo. `--verbose` also logs the scanned directories, the remotes read from each clone, and why each candidate matched or not.

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
