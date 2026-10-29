# Doctor

**Status:** agreed. Items marked **(leaning)** are proposals. Nothing is built.

`gero doctor` checks that a developer's workstation is set up to use `gero`, and says how to fix what is not. It is the first thing to run when something is off, and the output is meant to be pasted into an issue. Flags and the command tree are in [CLI_FRAMEWORK](CLI_FRAMEWORK.md); output rules are in [CLI_OUTPUT](CLI_OUTPUT.md).

Prior art is `rancher-deployer doctor`: a `Checker` with `Name`, `Category` and `Check`, a result with a message and a remediation, results grouped by category with a count and a closing verdict, exit 1 only on a failure. This spec keeps that shape and adds a `skip` status, `--json`, running named checks, and listing them.

## 1. Scope

* **Built for workstations.** The checks assume an engineer's machine with a user file and local clones. Nothing stops it from running in CI, and it has no CI-specific behavior, but no workflow is expected to call it: CI makes its own assumptions about the runner and has no user-level config. It is an either-context command with no gating ([CLI_FRAMEWORK §3](CLI_FRAMEWORK.md)).
* **Read-only.** It never changes anything and never prints a token. There is no `--fix`. A remediation is text a person reads, ideally with the exact command.
* **Offline by default.** Checks that need the network run only with `--online`. A plain `gero doctor` works on a plane and never makes a request.

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
| `git` | dependencies | no | `git` is not on `PATH`; `warn` when `user.name` or `user.email` is unset | `internal/git/gitcli` |
| `gh` | dependencies | no | never; `warn` when `gh` is not on `PATH`, since the keyring token fallback and `gh auth login` depend on it; reports the version when found | go-gh `pkg/auth` |
| `user-file` | configuration | no | the user file exists and does not parse, or has an unknown key | `internal/clicfg` |
| `config-dir` | configuration | no | no central config directory is found | `internal/workspace` |
| `config` | configuration | no | the application config does not validate; needs `config-dir` | `internal/cli/configvalidate` |
| `repos` | configuration | no | never; `warn` for each project with no clone or several equal ones; needs `config` | `internal/cli/configresolve` |
| `auth-token` | authentication | no | no token is found for the host; names the variables to set | `internal/github` |
| `auth-api` | authentication | yes | GitHub rejects the token; `warn` on a timeout; needs `auth-token` | `internal/cli/authstatus` |
| `rate-limit` | authentication | yes | never; `warn` when little of the limit is left; needs `auth-api` | go-github |

Checks arrive with the feature that depends on them, in the same change. Likely next: token permissions for opening PRs, access to the tracker repo and its issues, push access to each fork, and clones far behind upstream. A check for something that does not exist yet is not added.

## 3. Output

Grouped by category, in the order environment, dependencies, configuration, authentication. A header line gives the `gero` version. Markers are `✔` ok, `⚠` warn, `✗` fail and `○` skip, so the result reads the same without color. A remediation follows its line, indented, after `→`:

```
gero 0.4.0 (commit abc1234, built 2026-10-29)

Dependencies
  ✔ git: 2.51.0, user.name and user.email are set
  ⚠ gh: not found
    → gero reads a token from the gh keyring through it; install it from https://cli.github.com, or set GH_TOKEN

Configuration
  ✔ config-dir: /Users/me/work/orbs-automation/config (cwd)
  ✗ config: 2 problems in config/projects.yaml
    → run `gero config validate` for the details
  ○ repos: skipped, needs config

Authentication
  ✔ auth-token: environment variable GH_TOKEN
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
* `--list-checks` prints each check's name, category, whether it is online, and a one-line description, then exits.
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

A check calls the same code as the focused command and never parses its text output. Today `configvalidate`, `configresolve` and `authstatus` render a string; each needs a function that returns its data (problems, per-repo resolution, `authstatus.Status`), with the command's `Run` becoming that function plus a render step. That refactor is part of adding the check, under the existing tests. `gero auth status`, `config validate` and `config resolve repos` stay as the detailed, focused commands, and a doctor remediation points at them.

## 8. Testing

Follows [TESTING](TESTING.md).

* **Runner:** fake checks cover order, `Needs` skipping on a failed or skipped prerequisite, `--online` gating, a timeout becoming `warn`, an erroring check becoming `fail`, and named selection pulling in prerequisites.
* **Renderers:** exact-string tests for text and JSON, including that no token appears.
* **Each check:** a table test against fakes or a controlled environment (a temporary `PATH`, `GH_CONFIG_DIR` and `XDG_CONFIG_HOME`). No check test touches the network; the online checks take a transport or a fake.
* **Wiring:** `cmd/doctor_test.go` covers flags, `--list-checks`, an unknown name and the exit codes.

## Open questions

* Whether checks should run in parallel, and when. Leaning: sequential until it hurts.
* Whether a redacted, paste-friendly support mode is wanted (paths shortened, host names kept) or the default output is already safe enough. The output never contains a token, but it does contain local paths. Undecided; revisit when someone pastes it into an issue.
