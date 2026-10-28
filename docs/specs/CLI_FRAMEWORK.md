# CLI Framework

**Status:** agreed. Items marked **(leaning)** are proposals. Nothing is built yet.

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

**Structure.** The root command and global flags live in `cmd/`, which imports only cobra, the standard library, `cmd/...`, `internal/cli/...` and `internal/errs` (lint-enforced, see [PROJECT_LAYOUT](PROJECT_LAYOUT.md) section 6). Each command is its own package, `internal/cli/<cmd>/`, exporting a constructor that `cmd/` registers. A command's `RunE` parses flags, builds dependencies from `Settings`, and calls into domain packages in `internal/`. Commands hold no business logic, so everything is testable without cobra.

## 2. Command tree and global flags

Human-run commands are noun-verb chains, like `gh` (real parent commands such as `gero config validate`). CI-only commands are prefixed `ci:` and listed under a cobra help group, modeled on `ob-charts-tool`. **Context** is CI-only or either (IMPLEMENTATION §2).

| Command | Context | Notes |
|---|---|---|
| `gero config validate` | either | Runs without credentials |
| `gero config show --resolved` | either | Application config with defaults applied; `--json` supported |
| `gero lines resolve` | either | Read-only |
| `gero repo check-semver <version>` | either | Read-only; port of the shared `check-semver` script. Emits env-style output |
| `gero repo branch-tags` | either | Read-only; port of the shared `branch-tags.sh` script. Emits env-style output |
| `gero workflows sync` | either | Writes; honors `--dry-run` |
| `gero workflows verify` | either | Drift check **(leaning)**; replaces a free-standing `verify` |
| `gero ci:reconcile` | CI-only | Cron; the only reconciler entry point **(leaning)**. Named with the group prefix, listed under the CI-only help group |
| `gero dashboard` | either | Moonshot; the name may become `triage` |

Global (persistent) flags:

| Flag | Meaning |
|---|---|
| `--config-dir` | [CLI_CONFIG](CLI_CONFIG.md) section 4 |
| `--dry-run` | Commands that write (to disk, git or GitHub) compute and print what they would do, and change nothing. Read-only commands accept and ignore it. Default is off, with `verify` as the explicit check mode **(leaning)**. |
| `--json` | Machine output ([CLI_OUTPUT](CLI_OUTPUT.md) section 1); shorthand for `--output json`. `--output env` prints `KEY=value` lines for `$GITHUB_OUTPUT` and `$GITHUB_ENV` |
| `-v`, `--verbose` | Raises log level ([CLI_OUTPUT](CLI_OUTPUT.md) section 2); repeatable |
| `--no-color` | Disables color ([CLI_OUTPUT](CLI_OUTPUT.md) section 1) |
| `--version` | Prints the version and exits |

Per-command flags, such as the per-release override of `requiresPrerelease` (IMPLEMENTATION §4), are defined with their command and follow `--kebab-case`.

## 3. Execution-context gating

A command declares its context with a cobra annotation, for example `gero/context: ci-only`. Each CI-only command also joins the cobra help group `ci` ("CI-Only Commands:", defined in `internal/cli/groups`), so `gero --help` lists them apart from the human-run noun-verb commands. One shared `PersistentPreRunE` enforces it: CI-only commands require `GITHUB_ACTIONS=true` and a token, and otherwise fail fast with a clear error. The annotation also drives a marker in `--help`, so gating is declared once per command and never reimplemented inside `RunE`.

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
