# CLI Framework

**Status:** agreed, and implemented for the command tree's first commands. Items marked **(leaning)** are proposals; see Not built yet at the end for the gaps.

How `gero` is built as a CLI: framework, command tree, global flags, context gating and exit codes. Product design is in [ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md). Settings and their sources are in [CLI_CONFIG](CLI_CONFIG.md).

## 1. Framework

**cobra**, with a custom CLI-config loader ([CLI_CONFIG](CLI_CONFIG.md)). No viper.

| Option | Verdict |
|---|---|
| **cobra** + custom loader | **Chosen.** The same framework as `gh`, so command idioms match (VISION) and the ecosystem (completions, docs generation, `go-gh` examples) is mature. |
| cobra + viper | Not chosen, kept as the fallback. Viper brings a large transitive tree. It ignores unknown keys by default, so a typo in a user file fails silently. It has a global singleton, case-insensitive keys, type coercion, and quirks when a config value meets a flag default. Our CLI config is a handful of flat keys, which does not earn that cost. |
| urfave/cli v3 | Not chosen. Lighter, with per-flag env sources, but a smaller ecosystem and less aligned with `gh`. |
| stdlib `flag` | Not chosen. Nested subcommands, help and completions would be hand-rolled. |

**Viper adoption trigger.** Revisit only if CLI config grows past roughly ten keys, needs nested or per-command sections, needs more than one file format, or needs watching or remote sources. The swap stays local because everything downstream reads one resolved `Settings` struct from `internal/clicfg`.

**Structure.** The root command and global flags live in `cmd/`, which imports only cobra, the standard library, `cmd/...`, `internal/cli/...`, `internal/clicfg` and `internal/errs` (lint-enforced, see Enforcement in [PROJECT_LAYOUT](PROJECT_LAYOUT.md)). All cobra code stays in `cmd/`: the command tree, help text, flags, annotations and gating. A command's `RunE` there is glue only: it turns args, flags and the environment into plain values, calls the command's package `internal/cli/<cmd>/` (plain functions, no cobra), writes the output and returns the error. That package builds dependencies from `Settings` and calls into domain packages in `internal/`, so the logic is testable without cobra, and `cmd/` needs only wiring tests.

## 2. Command tree and global flags

Human-run commands are noun-verb chains, like `gh` (real parent commands such as `gero config validate`). CI-only commands are prefixed `ci:` and listed under a cobra help group, modeled on `ob-charts-tool`. **Context** is CI-only or either ([execution contexts](../IMPLEMENTATION.md#execution-contexts)).

| Command | Context | Notes |
|---|---|---|
| `gero config validate` | either | Runs without credentials |
| `gero config show --resolved` | either | Application config with defaults applied; `--json` supported |
| `gero config resolve repos [name|owner/name|.]` | either | Read-only; local path of each central-config repo and how it was found ([WORKSPACE](WORKSPACE.md)); `--json` supported. Prints its table before exiting 1 when a repo is unresolved or ambiguous |
| `gero cd <name\|owner/name>` | either (built for workstations) | Read-only; prints the local clone of a project, found only by the user file (`repos.paths`, else `repos.search`), for a shell function to `cd` into: a program cannot change its parent's directory. The project is always named, never implied. Exits 1 when the project is unknown or has no unique clone ([WORKSPACE](WORKSPACE.md)) |
| `gero shell init <bash\|zsh>` | either (built for workstations) | Read-only; prints the completion script and the `gero` function that makes `gero cd` change directory ([SHELL_SETUP](SHELL_SETUP.md)) |
| `gero shell install`, `gero shell uninstall` | either (built for workstations) | Writes the user's shell profile, inside one marked block, after asking (`--yes` to approve, `--shell`, `--profile`); honors `--dry-run` ([SHELL_SETUP](SHELL_SETUP.md)) |
| `gero auth status [--hostname host]` | either | Read-only; the host and token source gero would use (`GH_TOKEN`, `GITHUB_TOKEN`, gh config, gh keyring) and the login GitHub reports for the token; the token is never printed. Prints its result before exiting 1 when GitHub rejects the token. `--json` supported |
| `gero doctor [check...]` | either (built for workstations) | Read-only; checks the workstation setup and says how to fix what is not. Offline unless `--online`; `--json`, `--list-checks` and `--strict` ([DOCTOR](DOCTOR.md)) |
| `gero version` | either | Read-only; prints the build version, commit and date, the same text as `--version`. `--json` supported |
| `gero lines resolve` | either | Read-only |
| `gero semver check [tag]` | either | Read-only; port of the shared `check-semver` script. Takes a string (argument or `GITHUB_REF_NAME`) and never opens a repository. Emits env-style output |
| `gero repo info [name|owner/name]` | either | Read-only; a Project's config entry, its latest stable and pre-release tags and their commits read from the host (per line for a project with a branch per line), and its local clone and the clone's state. Defaults to the repository you are in. `--offline` skips the host, and a failed read or a missing clone leaves the rest of the report. `--json` supported |
| `gero repo images exist [name|owner/name] <tag>` | either | Read-only, needs the network; port of `validate_image_exists`. Checks that each image a Project publishes exists in the required registries ([IMAGES_EXIST](IMAGES_EXIST.md)). Prints its rows before exiting 1 when an image is missing or unreachable. `--json` supported |
| `gero repo branch-tags` | either | Read-only; port of the shared `branch-tags.sh` script. Emits env-style output |
| `gero ship [project] [tag|line]` | either | Writes; honors `--dry-run`. Downstream PRs: resolves the tag, checks the state of each consumer branch the release goes to (`--branch` for one), then edits, commits, pushes and opens a PR for each, superseding an older one ([PUSH_TO_REPO](PUSH_TO_REPO.md)). Today it stops after the state check and writes nothing. `--json` supported |
| `gero workflows sync` | either | Writes; honors `--dry-run` |
| `gero workflows verify` | either | Drift check **(leaning)**; replaces a free-standing `verify` |
| `gero ci:reconcile` | CI-only | Cron; the only reconciler entry point **(leaning)**. Named with the group prefix, listed under the CI-only help group |
| `gero dashboard` | either | Moonshot; the name may become `triage` |

A noun names the thing a command operates on. `gero repo` is for commands that read a repository (its refs, HEAD, tags), so repo selection (`--repo`, the inferred upstream and path in [CLI_CONFIG](CLI_CONFIG.md)) applies to everything under it. `gero semver` works on version strings and never opens a repository. The noun names the domain, not the implementation: `semver check` is the lenient script port (any text, one leading `v` stripped). Later verbs such as `compare` or `bump` may use a strict SemVer parser, which needs its own entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md); they get their own verbs and must not change what `check` accepts.

**Project arguments.** Wherever a command takes a project, it takes the config `name` or the repository's `owner/name` slug, case-insensitive, and `.` for the project whose repository the working directory is in. Where the project is optional, leaving it out means the same as `.`; the one exception is a command where leaving it out already means all projects (`gero config resolve repos`), which takes `.` explicitly. `gero cd` is the other exception: it always needs a name, and refuses `.`, since the directory you are in is where `.` would go. A command that takes a project and a release reads a lone argument as the release, with the project from the working directory: always for `gero repo images exist`, which needs a tag, and for `gero ship` when the argument looks like a version or a release line (`v0.5.3`, `v2.15`), since no project is named like that. One lookup does all of this for every command (`Workspace.Project`, [WORKSPACE](WORKSPACE.md)), so no command spells its own; outside a project the error says the working directory is not one.

Global (persistent) flags:

| Flag | Meaning |
|---|---|
| `--config-dir` | Where the central config is ([CLI_CONFIG](CLI_CONFIG.md)) |
| `--dry-run` | Commands that write (to disk, git or GitHub) compute and print what they would do, and change nothing: no branch is pushed and no PR is opened. Read-only commands accept and ignore it. Env `GERO_DRY_RUN`; not a user-file key, so a file cannot turn it on for every run. Default is off, with `verify` as the explicit check mode **(leaning)**. |
| `--non-interactive`, `--yes` | Never prompt (env `GERO_NON_INTERACTIVE`), and answer yes to the one confirmation a write command asks before it starts ([EXECUTION_CONTEXT](EXECUTION_CONTEXT.md#write-command-contract)). Without it, a write command with no TTY fails before writing **(leaning)** |
| `--json` | Machine output ([CLI_OUTPUT](CLI_OUTPUT.md)); shorthand for `--output json`. `--output env` prints `KEY=value` lines for `$GITHUB_OUTPUT` and `$GITHUB_ENV` |
| `-v`, `--verbose` | Raises log level ([CLI_OUTPUT](CLI_OUTPUT.md)): `-v` is `info`, `-vv` is `debug`. Wins over `GERO_LOG` |
| `--no-color` | Disables color ([CLI_OUTPUT](CLI_OUTPUT.md)) |
| `--version` | Prints the version and exits, with no shorthand. Root only, not persistent. The value comes from `cmd/version`, set by GoReleaser through `-ldflags -X` (`.goreleaser.yaml`); a plain `go build` prints `dev`. `gero version` is the primary form and prints the same text |

**Network policy.** A command that needs the network to do its job reads it, and does not ask first: an opt-in flag on every read would make the commands tedious to use. A command with a useful local-only mode takes `--offline` and says in its output what it did not read; making that a global flag is **(leaning)**. Reads that a public repository allows are made anonymously, and a token is sent only when the host asks for one. `gero doctor` is the exception, offline by default with `--online`, because it checks the workstation itself ([DOCTOR](DOCTOR.md)).

Per-command flags, such as the per-release override of `requiresPrerelease` ([release rules](../IMPLEMENTATION.md#release-rules)), are defined with their command and follow `--kebab-case`.

## 3. Execution-context gating

A command declares its context with a cobra annotation, for example `gero/context: ci-only`. Each CI-only command also joins the cobra help group `ci` ("CI-Only Commands:", defined in `cmd/groups`), so `gero --help` lists them apart from the human-run noun-verb commands. One shared `PersistentPreRunE` enforces it: CI-only commands require the CI context (`GITHUB_ACTIONS=true`, `CI=true` or `GERO_CONTEXT=ci`, so one can be run locally to debug) and a token, and otherwise fail fast with a clear error. The annotation also drives a marker in `--help`, so gating is declared once per command and never reimplemented inside `RunE`.

Commands that run in both contexts behave the same in each, except for who acts and where inputs come from. That, and the rules for commands that write, are in [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md).

## 4. Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Runtime error (I/O, GitHub, unexpected) |
| 2 | Usage or validation error (bad flag, invalid config) |
| 3 | `--dry-run` or `verify` found drift **(leaning)** |

One helper maps errors to codes so commands return errors and never call `os.Exit`.

## Not built yet

* The rest of the command tree beyond the commands in the table that do something today; the others are stubs.
* `--no-color`, context gating and the exit-code helper. Until the helper exists, a usage or validation error exits 1, not 2.
* `--json` for `config validate`, and `--output env` for `auth status`.

## Open questions

* Whether exit code 3 for drift is wanted, or `verify` output alone is enough.
* Final name for `gero dashboard` versus `triage`.
