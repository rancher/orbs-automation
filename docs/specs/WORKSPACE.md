# Workspace

**Status:** implemented for the central config, its directory, the current repository and local repo paths. Inference of the upstream, fork, `githubUser` and release line is agreed, not built.

`internal/workspace` is the one door from CLI settings to the world: the central config directory, the loaded application config, the current repository, and the local path, upstream and fork for a repo slug. Settings and their sources are in [CLI_CONFIG](CLI_CONFIG.md). The config it loads is in [APP_CONFIG](APP_CONFIG.md).

## One door

A command package may not locate the central config, discover a repo or scan for clones itself. Each rule exists once, in `workspace`, so the same inputs cannot resolve differently in different commands. Lint enforces both halves ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)): repositories are opened only through `workspace`, and the config loader (`internal/config/loader`) is importable only from `workspace`, so no command can see config that has skipped defaults or validation.

* Each `internal/cli/<cmd>` calls `workspace.Open(settings, cwd)`. It takes plain values and builds its own real dependencies (file system, go-git opener). A `getenv` joins the parameters when a rule first needs it, such as the CI check for `upstream`. `cmd/` knows nothing about `workspace`.
* The command's logic takes the `Workspace` through a small interface it defines, so tests inject a fake.
* **Resolution is lazy.** `Open` does no I/O, and a method does only the work its caller needs. A command that needs no remote does not fail because the working directory is not a repo.
* `internal/clicfg` stays pure (flags, environment, file, defaults; unset stays zero) and does not import git. `workspace` takes `Settings` and does the inference.
* The repository is offered as `Current()` (found from the working directory upward) and `CurrentAtRoot()` (`.git` must be in the working directory itself, for commands that port a script with that rule). `ErrNotRepository` is re-exported.

## Raw and effective values

A value in the central config can differ from the value an operation should act on. There are two layers, and each has one home:

* **Within the config.** A field omitted in the YAML is either filled at load, when a default is safe and conservative (`repo.defaultBranch`, `release.tagPattern`, and the others in [`config/README.md`](../../config/README.md)), or left empty and resolved by a getter on the type that owns the inputs, such as `Project.Branch(line)` (a development line with no branch lives on the default branch). The struct field is the **raw** value, exactly what was written, and validation reads it. A getter is the **effective** value, and anything that acts on a line reads that.
* **Config and CLI settings together.** A value that needs both, such as the upstream remote for one repo (a `repos.overrides` entry, else the global `upstream`), is a `Workspace` method. `clicfg` and `config` do not import each other, so only `workspace` can combine them. The rule for these is **replace, not merge**: the central config carries a CI-oriented default, and a user setting for the same thing replaces it whole. `pullRequests.branchPrefix` (a template of simple placeholders such as `{source}`) is replaced entirely by the user's `branchPrefix`, and what the user leaves out or adds is their call. Nothing in `gero` may depend on a branch name's shape, so a replacement cannot break it; an unknown placeholder is rejected.

`Workspace.Config()` and `Workspace.Project()` return config with defaults applied and validated; the loader is not reachable from anywhere else. A field with a fallback says so in its comment and names its getter, and a test over the config types fails when a field is added without being classed as plain, defaulted at load or resolved by a getter.

## Inferred defaults

Most settings can be left unset because they are inferred from the repo at the working directory, its remotes and its branch. Full precedence per value:

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
* A user-file value always beats inference.
* `gero config show` reports each value's source (`flag`, `env`, `file`, `inferred (reason)`, `default`), and verbose mode logs inferred values before any write (push, PR).
* `gero config show` may report one slug's resolved path with an `inferred (reason)` source, but it never enumerates repos. When `repos.search` is set, the text output ends with a line pointing at `gero config resolve repos`. Json and env never carry it.

## Resolving repo paths

`gero config resolve repos [slug]` answers "where does each repo live on this machine, and how did `gero` decide?". `config show` reports raw values and their sources. This command runs the same resolver (`Workspace.RepoPath`) the other commands use, so its output is what they would act on. The slugs come from `Workspace.Config()`, so an invalid config is rejected before anything is scanned. It is read-only and supports `--json`.

**Order for one slug:** the CWD repo if one of its remotes is the slug (a fork alone does not count), then `repos.paths`, then the `repos.search` scan, then the flat sibling. A `repos.paths` entry is returned as written, without checking that it exists. The sibling is `<parent of the CWD repo's root>/<name>` and counts only when that directory exists and the working directory is in a repo. Paths derived from the CWD repo's root come back with symlinks resolved, as go-git reports them (`/tmp` is `/private/tmp` on macOS).

**The scan.** Each `repos.search` directory is scanned to depth 2 for clones. A directory is a clone if it contains a `.git` directory (worktrees and submodules, where `.git` is a file, are not found), and the scan does not look inside a clone. Remote URLs are read from the repository config through `internal/git/repo` (go-git, no `git` subprocess) and parsed to `owner/name` slugs (`git.SlugFromURL`: https, ssh and scp-like URLs, host not checked, local paths ignored). Directory names are never used for matching, so no layout is assumed, and slugs compare case-insensitively.

* A clone with a remote that is the slug itself, whatever the remote is called, is a **strong** match.
* A clone that only has a fork of the slug (same name, different owner) is a **weak** match and counts only when no strong match exists.
* Several matches of the same strength are **ambiguous**, never picked silently.

**Cost.** Most commands resolve one slug, usually from the CWD repo or `repos.paths`, and never scan. The scan runs only when those miss, and it is built once per command and matched against every slug asked for. Resolving all slugs is this command's case.

**Output.** One row per `Project` in the central config. `External` projects get no rows: no command clones them today, and they can be added once one needs a local path. Each row has the path and how it was found: `cwd`, `repos.paths`, `repos.search scan` or `sibling`.

* Unresolved and ambiguous slugs are rows, not errors. Text output keeps them short, `(not found)` or `(ambiguous: <candidate paths>)`, with a source of `-`, and ends with one hint line for each kind of problem present: clone the repo under a `repos.search` directory or set `repos.paths.<slug>` (not found), or set `repos.paths.<slug>` to choose one (ambiguous).
* Json is an array of `{slug, path, source, candidates, error}` objects and carries no hints. Other output formats (`env`) are an error.
* The command exits non-zero if any row is unresolved or ambiguous, after printing the table.
* An optional `<slug>` argument limits the output to that `Project` (spelled as the config spells it) and is an error for a slug that is not a `Project`.

## Not built yet

* Inference of the upstream, fork, `githubUser` and release line from remotes, and the `inferred` source in `config show`.
* `config resolve repos --verbose` **(leaning)**: log the scanned directories, the remotes read from each clone, and why each candidate matched or not.

## Open questions

None yet.
