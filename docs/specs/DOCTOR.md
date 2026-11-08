# Doctor

**Status:** implemented. Items marked **(leaning)** are proposals; see Not built yet at the end for the gap.

`gero doctor` checks that a developer's workstation is set up to use `gero`, and says how to fix what is not. It is the first thing to run when something is off, and the output is meant to be pasted into an issue. Flags and the command tree are in [CLI_FRAMEWORK](CLI_FRAMEWORK.md); output rules are in [CLI_OUTPUT](CLI_OUTPUT.md).

Prior art is `rancher-deployer doctor`: a `Checker` with `Name`, `Category` and `Check`, a result with a message and a remediation, results grouped by category with a count and a closing verdict, exit 1 only on a failure. This spec keeps that shape and adds a `skip` status, `--json`, running named checks, and listing them.

## 1. Scope

* **Built for workstations.** The checks assume an engineer's machine with a user file and local clones. Nothing stops it from running in CI, and it has no CI-specific behavior, but no workflow is expected to call it: CI makes its own assumptions about the runner and has no user-level config. It is an either-context command with no gating ([CLI_FRAMEWORK §3](CLI_FRAMEWORK.md)).
* **Read-only.** It never changes anything and never prints a token. There is no `--fix`. A remediation is text a person reads, ideally with the exact command.
* **Offline by default.** Checks that need the network run only with `--online`. A plain `gero doctor` works on a plane and never makes a request.
* **It runs when the user file is broken.** Every other command stops on a user file it cannot load. When loading fails, `gero doctor` loads its settings again without the file, so it can report the problem in `user-file`. A bad flag or environment value still stops it: only the file is excused.

## 2. Checks

A check is one question with one answer. It implements a small interface, defined in `internal/cli/doctor` where it is consumed:

* `Name()`: a short stable kebab-case name (`git`, `auth-api`), used on the command line and in JSON.
* `Category()`: one of the groups in section 3.
* `Online()`: whether it needs the network.
* `Needs()`: names of checks that must be `ok` or `warn` first. If one failed or was skipped, this check is `skip` with the reason. This keeps one missing token from printing five more failures.
* `Run(ctx)`: returns a `Result`.

A `Result` is a status, a one-line message, and an optional remediation. Statuses:

| Status | Meaning |
|---|---|
| `ok` | Works as expected |
| `warn` | Works, but something may bite later, or an optional piece is missing |
| `fail` | `gero` cannot do its job until this is fixed |
| `skip` | Not run: it needs `--online`, or a prerequisite check did not pass |

Checks are independent. One never stops the others, and none panics the run: an error inside a check is a `fail` with the error as its message. Checks run in a fixed order, one at a time **(leaning)**; the online ones may run in parallel later if the list grows.

Every `warn` and `fail` carries a remediation that says why it matters when that is not obvious, and what to do.

### The first set

Each one reuses the package that already does the work (section 7).

| Check | Category | Online | Fails when | Reuses |
|---|---|---|---|---|
| `version` | environment | no | never; reports the version, commit, OS and architecture | `cmd/version` |
| `git` | dependencies | no | `git` is not on `PATH` or will not report a version, since `gero repo build-tags` runs it; `warn` when `user.name` or `user.email` is unset, since commits gero makes will need them. It asks git itself, so includes and system config count; bounds its calls so a hung `git` cannot hang the run | `internal/platform/git/gitcli` (`Version`, `Config`) |
| `gh` | dependencies | no | never; `warn` when `gh` is not on `PATH`, since the keyring token fallback and `gh auth login` depend on it; reports the version and path when found; bounds `gh --version` so a hung `gh` cannot hang the run | `internal/platform/ghcli`, which finds `gh` the way go-gh does (`GH_PATH`, then `PATH`) |
| `user-file` | configuration | no | the user file exists and a command would refuse it: it does not parse, has an unknown key, or holds a value such as a relative path. It is judged on its own, with the `GERO_*` variables masked, so a bad environment value is not blamed on the file. With no file it is `ok`, saying where one would go | `internal/clicfg` (`OpenUserFile`, `Load`) |
| `config-dir` | configuration | no | no central config directory is found; the fix is the advice in `workspace.NotFoundHint` | `internal/workspace` (`ConfigDir`) |
| `config` | configuration | no | the application config does not validate, counting the problems the validator found; needs `config-dir` | `internal/workspace` (`Config`) |
| `repos` | configuration | no | never; `warn` when a project has no local clone or several equally good ones, with the fix for each kind; needs `config` | `internal/workspace` (`RepoPaths`), and the fix text from `internal/cli/configresolve` |
| `auth-token` | authentication | no | no token is found for the default host; names the variables to set; reports where the token came from and never the token. It runs the same resolver as every command, so it may call `gh auth token` | `internal/platform/github` (`Resolve`, `NoTokenError`, `DescribeTokenSource`) |
| `auth-api` | authentication | yes | GitHub rejects the token, with advice that depends on where the token came from (replace an environment variable, or `gh auth login` again); `warn` on a timeout or any other error, since a network problem says nothing about the token; needs `auth-token` | `internal/cli/authstatus` (`Check`) |
| `rate-limit` | authentication | yes | never; `warn` when under a tenth of the core limit is left, saying when the window resets, or when the limit cannot be read; needs `auth-api` | `internal/platform/github` (`CoreRateLimit`) |
| `commit-identity` | authentication | in CI only | never; `warn` when no identity resolves (no `user.name`/`user.email` on a workstation, no `GERO_BOT_USER` or an unknown bot login in CI), since read-only commands need none; reports who commits would be attributed to; in CI needs `auth-api` | `internal/cli/identity` (`Resolve`) |

Checks arrive with the feature that depends on them, in the same change. Likely next: token permissions for opening PRs, access to the tracker repo and its issues, push access to each fork, and clones far behind upstream. A check for something that does not exist yet is not added.

## 3. Output

Grouped by category, in the order environment, dependencies, configuration, authentication. A header line gives the `gero` version. Markers are `✔` ok, `⚠` warn, `✗` fail and `○` skip, so the result reads the same without color. With color on ([CLI_OUTPUT](CLI_OUTPUT.md#terminal-capabilities)) the markers, headings, remediation and closing line are styled, and `--json` is never styled. A remediation follows its line, indented, after `→`:

```
gero 0.4.0 (commit abc1234, built 2026-10-29)

Dependencies
  ✔ git: 2.51.0, user.name and user.email are set
  ⚠ gh: gh not found on PATH
    → gero reads a token from the gh keyring through it: install it from https://cli.github.com, or set GH_TOKEN (or fix GH_PATH, if you set it)

Configuration
  ✔ user-file: /Users/me/.config/gero/config.yaml
  ✔ config-dir: /Users/me/work/orbs-automation/config (cwd)
  ✗ config: 2 problems in the central config
    → run `gero config validate` for the details
  ○ repos: skipped, needs config

Authentication
  ✔ auth-token: environment variable GH_TOKEN for github.com
  ○ auth-api: skipped, run with --online

Summary: 3 ok, 1 warning, 1 failed, 2 skipped
1 check needs --online. Some checks failed: fix the errors above, then run gero doctor again.
```

The closing line is one of: all ok (`Your workstation is ready to use gero.`), warnings only (`Ready, with warnings.`), or failures (as above). It mentions how many checks need `--online` when any were skipped for that.

`--json` prints one object: `gero` (version, commit, date, os, arch), `checks` (each with `name`, `category`, `online`, `status`, `message`, `remediation`, and `skipReason` when skipped) and `summary` (counts). Field names are stable. `--output env` is not supported.

## 4. Command line

```
gero doctor [check...] [--online] [--timeout 10s] [--strict] [--list-checks]
```

* `gero doctor <check>...` runs only the named checks, plus the checks they need, which are shown. Naming an online check runs it without `--online`: asking for it by name is the opt-in.
* `--list-checks` prints each check's name, category, whether it is online, and a one-line description, then exits; it honors `--json` too.
* `--online` adds the checks that need the network. `--timeout` bounds each of them. A timeout is a `warn`, never a `fail`, since a flaky network is not a broken setup.
* `--strict` makes warnings exit non-zero.
* An unknown check name is a usage error and prints the list.

## 5. Exit codes

Following [CLI_FRAMEWORK §4](CLI_FRAMEWORK.md):

| Code | When |
|---|---|
| 0 | No `fail`. Warnings and skips are fine |
| 1 | At least one `fail`, or a `warn` with `--strict` |
| 2 | Usage error, such as an unknown check name |

## 6. Layout

`internal/cli/doctor` holds the check interface, the runner (order, `Needs`, timeouts, error-to-fail), and the renderers. Checks live in the same package, one file per category. `cmd/doctor.go` is glue: it turns flags into plain values, calls the package, writes the output and returns the error, as every command does ([PROJECT_LAYOUT](PROJECT_LAYOUT.md)).

## 7. Reuse

A check calls the code behind the focused command and never parses its text output. The configuration checks ask the workspace, the one door to the central config and the local clones ([WORKSPACE](WORKSPACE.md)), through a small interface the doctor defines. The authentication checks use `authstatus.Check`, which returns the result as data, and `gero auth status` is that plus a render step. The focused commands (`auth status`, `config validate`, `config resolve repos`) stay as the detailed views, and a remediation points at them.

## 8. Testing

Follows [TESTING](TESTING.md).

* **Runner:** fake checks cover order, `Needs` skipping on a failed or skipped prerequisite, `--online` gating, a timeout becoming `warn`, an erroring check becoming `fail`, and named selection pulling in prerequisites.
* **Renderers:** exact-string tests for text and JSON, including that no token appears.
* **Each check:** a table test against fakes or a controlled environment (a temporary `PATH`, `GH_CONFIG_DIR` and `XDG_CONFIG_HOME`). No check test touches the network; the online checks take a transport or a fake.
* **Wiring:** `cmd/doctor_test.go` covers flags, `--list-checks`, an unknown name and the exit codes.

## Not built yet

* The exit-code helper: until it exists a usage error, such as an unknown check name, exits 1, not 2.

## Open questions

* Whether checks should run in parallel, and when. Leaning: sequential until it hurts.
* Whether a redacted, paste-friendly support mode is wanted (paths shortened, host names kept) or the default output is already safe enough. The output never contains a token, but it does contain local paths. Undecided; revisit when someone pastes it into an issue.
