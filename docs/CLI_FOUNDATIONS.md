# CLI Foundations

Decisions for the layer under the application: how `gero` is built as a CLI, how a single invocation gets its settings, and how it talks to the user. Product design is in [ARCHITECTURE_DESIGN](ARCHITECTURE_DESIGN.md), GitHub access and release rules are in [IMPLEMENTATION](IMPLEMENTATION.md), and the milestones that build this are in [PHASES](PHASES.md) (v0.1).

Status markers follow IMPLEMENTATION: unmarked items are decisions, **(leaning)** is a proposal not yet agreed, **(unverified)** needs a small spike before we rely on it.

## 1. Terms

Two kinds of config exist, and the docs keep them apart.

* **Application config** describes the world `gero` operates on: projects, Rancher lines, external projects, and per-repo workflow settings. It is the multi-document YAML in `config/` and a target repo's `gero.yaml` (IMPLEMENTATION §7). Versioned by `apiVersion`.
* **CLI config** describes how one invocation behaves: where to find the application config, which remote counts as upstream, output format, verbosity, host. It comes from flags, environment variables and an optional user file (section 4). It never holds application facts, and it never holds secrets.

Code mirrors the split: `internal/config` for application config, `internal/clicfg` for CLI config.

## 2. Dependency policy

* **Standard library first.** Fewer dependencies means a smaller supply-chain surface and less to audit.
* A new dependency needs an entry in this section (or the IMPLEMENTATION section it belongs to) saying what the standard library would cost us. Enforced by a `depguard` allow-list in `.golangci.yml`, so an unlisted import fails lint.
* Approved now:

| Module | For |
|---|---|
| `github.com/spf13/cobra` | Command tree, flags, help, completions (section 3) |
| `github.com/cli/go-gh` | GitHub auth, host resolution, HTTP client (IMPLEMENTATION §3) |
| `github.com/google/go-github` | Typed REST (IMPLEMENTATION §3) |
| One YAML library | Application config and the user file. **(unverified)** Pick one that supports multi-document decoding and rejecting unknown fields, and record it here. |

* Not adopted: `viper`, `testify`, color or logging libraries, any TUI library (sections 3, 7, 8, 10, 11).

## 3. Framework

**cobra**, with a custom CLI-config loader (section 4). No viper.

| Option | Verdict |
|---|---|
| **cobra** + custom loader | **Chosen.** The same framework as `gh`, so command idioms match (VISION) and the ecosystem (completions, docs generation, `go-gh` examples) is mature. |
| cobra + viper | Not chosen, kept as the fallback. Viper brings a large transitive tree. It ignores unknown keys by default, so a typo in a user file fails silently. It has a global singleton, case-insensitive keys, type coercion, and quirks when a config value meets a flag default. Our CLI config is a handful of flat keys, which does not earn that cost. |
| urfave/cli v3 | Not chosen. Lighter, with per-flag env sources, but a smaller ecosystem and less aligned with `gh`. |
| stdlib `flag` | Not chosen. Nested subcommands, help and completions would be hand-rolled. |

**Viper adoption trigger.** Revisit only if CLI config grows past roughly ten keys, needs nested or per-command sections, needs more than one file format, or needs watching or remote sources. The swap stays local because everything downstream reads one resolved `Settings` struct from `internal/clicfg`.

**Structure.** One file per command group under `cmd/gero/`. A command's `RunE` parses flags, builds dependencies from `Settings`, and calls into `internal/`. Commands hold no business logic, so everything is testable without cobra.

## 4. CLI config: sources and precedence

Highest wins:

1. **Flags**, detected with `cmd.Flags().Changed()` so a flag's default never masks a lower source.
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
| `configDir` | `GERO_CONFIG_DIR` | `--config-dir` | Where the central application config is (section 5) |
| `upstream` | `GERO_UPSTREAM` | `--upstream` | Remote or repo treated as the real source |
| `output` | `GERO_OUTPUT` | `--output`, `--json` | `text` (default) or `json` |
| `host` | `GERO_HOST` | `--host` | GitHub host, if we ever target non-github.com (IMPLEMENTATION §3) |
| n/a | `GERO_LOG` | `-v` | Log level (section 8) |
| n/a | `GERO_CONFIG` | n/a | Path of the user file |

Env and flag names derive from the file key by one rule, so the table is a check, not a source of truth. The loader is table-tested for precedence and for unknown keys.

## 5. Locating the central application config

Commands that read the central `config/` find it in this order:

1. `--config-dir`
2. `GERO_CONFIG_DIR`
3. `configDir` in the user file
4. `./config`, when the working directory is a checkout of the central repo

If none resolves, the command fails with a message listing these four places. There is no runtime fetch of config from another branch (IMPLEMENTATION §7, "Avoid"). In CI the central repo is checked out and `GERO_CONFIG_DIR` or `--config-dir` is set explicitly in the workflow.

## 6. Command tree and global flags

Noun-verb, like `gh`. **Context** is CI-only or either (IMPLEMENTATION §2).

| Command | Context | Notes |
|---|---|---|
| `gero config validate` | either | Runs without credentials |
| `gero config show --resolved` | either | Application config with defaults applied; `--json` supported |
| `gero lines resolve` | either | Read-only |
| `gero workflows sync` | either | Writes; honors `--dry-run` |
| `gero workflows verify` | either | Drift check **(leaning)**; replaces a free-standing `verify` |
| `gero reconcile` | CI-only | Cron; the only reconciler entry point **(leaning)**: kept top-level as it is the product's verb |
| `gero dashboard` | either | Moonshot; the name may become `triage` |

Global (persistent) flags:

| Flag | Meaning |
|---|---|
| `--config-dir` | Section 5 |
| `--dry-run` | Commands that write (to disk, git or GitHub) compute and print what they would do, and change nothing. Read-only commands accept and ignore it. Default is off, with `verify` as the explicit check mode **(leaning)**. |
| `--json` | Machine output (section 7); shorthand for `--output json` |
| `-v`, `--verbose` | Raises log level (section 8); repeatable |
| `--no-color` | Disables color (section 7) |
| `--version` | Prints the version and exits |

Per-command flags, such as the per-release override of `requiresPrerelease` (IMPLEMENTATION §4), are defined with their command and follow `--kebab-case`.

### Execution-context gating

A command declares its context with a cobra annotation, for example `gero/context: ci-only`. One shared `PersistentPreRunE` enforces it: CI-only commands require `GITHUB_ACTIONS=true` and a token, and otherwise fail fast with a clear error. The annotation also drives a marker in `--help`, so gating is declared once per command and never reimplemented inside `RunE`.

## 7. Output

* **Data to stdout, diagnostics to stderr.** A script can always pipe stdout.
* Human output uses `text/tabwriter` and plain lines. No table or styling library.
* Every command that emits data supports `--json` through `encoding/json`, with a stable field naming scheme documented next to the command. JSON output is never interleaved with human text.
* Color is off when `NO_COLOR` is set, `--no-color` is passed, or stdout is not a TTY. No color library until output needs more than a few escape codes.

## 8. Logging

Standard library `log/slog`, written to stderr. Default level `warn`, `-v` for `info`, `-vv` for `debug`, or `GERO_LOG=debug|info|warn|error`. A text handler locally and a JSON handler when running in CI. Business code receives a `*slog.Logger`; there is no package-level logger.

## 9. Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Runtime error (I/O, GitHub, unexpected) |
| 2 | Usage or validation error (bad flag, invalid config) |
| 3 | `--dry-run` or `verify` found drift **(leaning)** |

One helper maps errors to codes so commands return errors and never call `os.Exit`.

## 10. TUI and interactive features

Deferred. Nothing in v0.x or v1.0.0 needs prompts beyond handing off to `gh auth login`.

* Core commands stay non-interactive and scriptable.
* Any prompt or rich rendering goes through one internal package, `internal/ui`, behind an interface with a plain-text implementation first. This is the seam that lets a TUI arrive later without touching commands.
* Interactive behavior is opt-in and automatically disabled when stdin or stdout is not a TTY, or when running in CI.
* The likely future stack is the Charm libraries (bubbletea, huh, lipgloss). No dependency is added until a feature needs it, and the addition then gets an entry in section 2. The web dashboard (ARCHITECTURE_DESIGN §6) is a separate, later surface.

## 11. Testing the CLI layer

* Build the root command in a test, run it in process with captured stdout and stderr, and assert on output and exit code. Golden files live under `testdata/`.
* Standard library `testing` only; no `testify` for now.
* `internal/clicfg` is table-tested: each source alone, each pairwise precedence, unknown key in the user file, malformed file, missing file, CI ignoring the user file.
* Gating is tested by running a CI-only command with and without `GITHUB_ACTIONS` and a token.
* Follows the commit workflow in [`ai/red-green-commits.md`](ai/red-green-commits.md).

## 12. Planned layout

```
cmd/gero/            cobra wiring only: root, global flags, one file per command group
internal/clicfg/     CLI config loader -> Settings
internal/config/     application config: types, loader, validation, defaults
internal/ui/         output, prompts, color; plain-text implementation first
internal/...         domain packages as they arrive (lines, bump, reconcile, github)
```

Package names beyond these four are decided with the code that needs them.

## 13. Open questions

* Which YAML library (section 2).
* Whether `--dry-run` should become the default for commands that write to GitHub.
* Whether exit code 3 for drift is wanted, or `verify` output alone is enough.
* Final names for the user-file keys and for `gero dashboard` versus `triage`.
