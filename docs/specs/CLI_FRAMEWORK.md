# CLI Framework

**Status:** agreed. Items marked **(leaning)** are proposals. Built so far: the cobra root, `--version`, the global flags `--config-dir`, `--upstream`, `--output` and `--json` (resolved into `clicfg.Settings` in the root `PersistentPreRunE`), `semver check` and `repo branch-tags`. The rest of the command tree is stubs, and `--dry-run`, `-v`, `--no-color`, gating and exit codes are not built. Only `config show` and `repo branch-tags` read `Settings`, the latter through `internal/workspace` for the repository.

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

**Structure.** The root command and global flags live in `cmd/`, which imports only cobra, the standard library, `cmd/...`, `internal/cli/...`, `internal/clicfg` and `internal/errs` (lint-enforced, see [PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 6). All cobra code stays in `cmd/`: the command tree, help text, flags, annotations and gating. A command's `RunE` there is glue only: it turns args, flags and the environment into plain values, calls the command's package `internal/cli/<cmd>/` (plain functions, no cobra), writes the output and returns the error. That package builds dependencies from `Settings` and calls into domain packages in `internal/`, so the logic is testable without cobra, and `cmd/` needs only wiring tests.

## 2. Command tree and global flags

Human-run commands are noun-verb chains, like `gh` (real parent commands such as `gero config validate`). CI-only commands are prefixed `ci:` and listed under a cobra help group, modeled on `ob-charts-tool`. **Context** is CI-only or either (IMPLEMENTATION §2).

| Command | Context | Notes |
|---|---|---|
| `gero config validate` | either | Runs without credentials |
| `gero config show --resolved` | either | Application config with defaults applied; `--json` supported |
| `gero config resolve repos [slug]` | either | Read-only; local path of each central-config repo and how it was found ([CLI_CONFIG](CLI_CONFIG.md) section 3b); `--json` supported |
| `gero lines resolve` | either | Read-only |
| `gero semver check [tag]` | either | Read-only; port of the shared `check-semver` script. Takes a string (argument or `GITHUB_REF_NAME`) and never opens a repository. Emits env-style output |
| `gero repo branch-tags` | either | Read-only; port of the shared `branch-tags.sh` script. Emits env-style output |
| `gero workflows sync` | either | Writes; honors `--dry-run` |
| `gero workflows verify` | either | Drift check **(leaning)**; replaces a free-standing `verify` |
| `gero ci:reconcile` | CI-only | Cron; the only reconciler entry point **(leaning)**. Named with the group prefix, listed under the CI-only help group |
| `gero dashboard` | either | Moonshot; the name may become `triage` |

A noun names the thing a command operates on. `gero repo` is for commands that read a repository (its refs, HEAD, tags), so repo selection (`--repo`, the inferred upstream and path in [CLI_CONFIG](CLI_CONFIG.md)) applies to everything under it. `gero semver` works on version strings and never opens a repository. The noun names the domain, not the implementation: `semver check` is the lenient script port (any text, one leading `v` stripped). Later verbs such as `compare` or `bump` may use a strict SemVer parser, which needs its own entry in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md); they get their own verbs and must not change what `check` accepts.

Global (persistent) flags:

| Flag | Meaning |
|---|---|
| `--config-dir` | [CLI_CONFIG](CLI_CONFIG.md) section 4 |
| `--dry-run` | Commands that write (to disk, git or GitHub) compute and print what they would do, and change nothing. Read-only commands accept and ignore it. Default is off, with `verify` as the explicit check mode **(leaning)**. |
| `--json` | Machine output ([CLI_OUTPUT](CLI_OUTPUT.md) section 1); shorthand for `--output json`. `--output env` prints `KEY=value` lines for `$GITHUB_OUTPUT` and `$GITHUB_ENV` |
| `-v`, `--verbose` | Raises log level ([CLI_OUTPUT](CLI_OUTPUT.md) section 2); repeatable |
| `--no-color` | Disables color ([CLI_OUTPUT](CLI_OUTPUT.md) section 1) |
| `--version` | Prints the version and exits. No `-v` shorthand, which is `--verbose`. The value comes from `cmd/version`, set by GoReleaser through `-ldflags -X` (`.goreleaser.yaml`); a plain `go build` prints `dev` |

Per-command flags, such as the per-release override of `requiresPrerelease` (IMPLEMENTATION §4), are defined with their command and follow `--kebab-case`.

## 3. Execution-context gating

A command declares its context with a cobra annotation, for example `gero/context: ci-only`. Each CI-only command also joins the cobra help group `ci` ("CI-Only Commands:", defined in `cmd/groups`), so `gero --help` lists them apart from the human-run noun-verb commands. One shared `PersistentPreRunE` enforces it: CI-only commands require `GITHUB_ACTIONS=true` and a token, and otherwise fail fast with a clear error. The annotation also drives a marker in `--help`, so gating is declared once per command and never reimplemented inside `RunE`.

## 4. Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Runtime error (I/O, GitHub, unexpected) |
| 2 | Usage or validation error (bad flag, invalid config) |
| 3 | `--dry-run` or `verify` found drift **(leaning)** |

One helper maps errors to codes so commands return errors and never call `os.Exit`.

## Open questions

* Whether `--dry-run` should become the default for commands that write to GitHub.
* Whether exit code 3 for drift is wanted, or `verify` output alone is enough.
* Final name for `gero dashboard` versus `triage`.
