# Project Layout

**Status:** implemented, except the prompt side of `internal/ui`, two of the tasks in `internal/task/`, and `test/integration/` (see Not built yet).

How Go code is organized in this repo. `gero` is a CLI, not a library, so the default is that nothing is importable from outside the module.

## 1. The `internal/` rule

* All Go packages live under `internal/`, except the `main` package (`main.go` at the repo root) and the cobra layer under `cmd/`. The compiler enforces that code outside this module cannot import `internal/`.
* There is no `pkg/` directory. Moving a package out of `internal/` later is cheap. Retracting a public API is not.
* **Promotion criteria.** A package leaves `internal/` only when all hold: a real external consumer exists, its API has survived at least one minor release unchanged, and it has no dependency on other `internal/` packages that are not themselves promoted. Promotion is a decision recorded in this spec, not a drive-by move.
* Data files (`config/`, `config-mock/`, `mock-repos/`) are not Go code and stay at the top level.

## 2. Layout

```
main.go              entry point: calls cmd.Execute(), nothing else
cmd/                 all cobra code: root command, command tree, flags, help text, groups, gating; imports only internal/cli, internal/clicfg and internal/errs
cmd/version/         build info (Version, Commit, Date) set by GoReleaser through -ldflags -X; read only by cmd/, passed down as a value
internal/cli/<cmd>/  one subpackage per command: its logic as plain functions, no cobra
internal/cli/logging/ shared, not a command: builds the *slog.Logger, called once by cmd/   (CLI_OUTPUT)
internal/clicfg/     CLI config loader -> Settings            (CLI_CONFIG)
internal/config/     application config types, their effective getters and the enum values
internal/config/loader/  application config loader: YAML decoding, defaults, validation (workspace only)
internal/workspace/  the one door from Settings to the world: config dir, loaded config, repo paths, upstream, fork   (WORKSPACE)
internal/ui/         output and prompt seam, plain-text implementation first: Output and Stream exist, prompts do not yet   (CLI_OUTPUT)
internal/task/<name>/  one package per single action (create a branch, bump and commit, push a branch, open a PR), run by a command alone or chained by a higher-order one such as ship   (see Tasks below); pushbranch, createpr and createbranch are built
internal/git/        pure logic about git refs, tags and commits (branch tags, commit abbreviation); no go-git, no exec
internal/git/repo/   local repository access with go-git; the only importer of go-git
internal/execx/      the one place gero starts another program: allow-listed binaries, raw stdout, stderr and exit code, no formatting; the only importer of os/exec
internal/git/gitcli/ escape hatches to the git binary, one documented function per thing go-git cannot do; runs git through execx
internal/ghcli/      finds and runs the gh binary for gero doctor (version only); runs gh through execx
internal/github/pulls/  the PR service: find, create, comment on, close and label pull requests, in plain types; where go-github's pull request calls stay
internal/workdir/   the branch a run edits and commits in: made in the engineer's clone or a temporary worktree of it, committed to, and put back when the work ends or fails; shared by the tasks that make a branch and the ones that commit to it
internal/ship/      one downstream PR: the state check and the write sequence, with its interfaces   (PUSH_TO_REPO). It still holds most of the actions; they move to internal/task/ and ship keeps the decisions and the order
internal/ship/templates/  the embedded commit, body and title templates and their placeholder renderer
internal/updatedispatcher/  applies a bump: strategies claimed by name, the engine built from config, and the step lifecycle around the edit; starts no process   (PUSH_TO_REPO)
internal/yamledit/  changes one YAML value in place without reflowing the file; the third importer of the YAML library
internal/...         domain packages as they arrive: lines, reconcile, github
test/integration/    separate Go module (own go.mod) for integration tests     (TESTING)
```

**Decision: cobra lives only in `cmd/`, command logic lives in `internal/cli/`.** `cmd/` (package `cmd`, with `main.go` at the repo root, as `cobra-cli init` generates) is the one place that knows about cobra. It holds the root command, the global flags, the command tree, each command's `Use`, help text, flags and annotations, the help groups, the context gating, and a short `RunE` per command. That `RunE` only translates: it turns args, flags and the environment into plain values, calls a function in `internal/cli/<cmd>/`, writes the result to `cmd.OutOrStdout()` and returns the error. It may import the standard library, cobra, its own subpackages (`cmd/...`), `internal/cli/...`, `internal/clicfg` and `internal/errs` (exit-code errors, needed to wire `Execute`). It may **not** import any other `internal/` package. Its use of `internal/clicfg` is narrow: it builds a `clicfg.FlagLookup` over the parsed flags, calls `clicfg.Load` once in the root `PersistentPreRunE`, and passes the resulting `Settings` down by value.

`internal/cli/<cmd>/` holds everything a command does beyond that translation, as plain exported functions that never import cobra. Inputs are plain values: strings, an options struct for flags, and a `getenv func(string) string` instead of `os.Getenv`. Outputs are returned values (a string, a struct) or an `io.Writer` argument, plus errors, with sentinel errors where a caller needs to tell them apart. Example: `gero semver check` is `checksemver.ResolveTag(args, getenv) (string, error)`, `checksemver.Classify(tag) semver.Info` and `checksemver.Render(info) string`, called from the `RunE` in `cmd/repo.go`. A noun command such as `gero repo` is a parent in `cmd/`; each child (`gero semver check`, `gero repo branch-tags`) keeps its own `internal/cli/<child>/` package, named for its logic (`checksemver`, `branchtags`).
**CI-only commands.** The help group is still a `cobra.Group` ("CI-Only Commands:") following `ob-charts-tool`. Its definition and the `gero/context` annotation constants live in `cmd/groups/`. A CI-only command prefixes its `Use` with the group ID (`gero ci:reconcile`) and sets the annotation and `GroupID` where `cmd/` defines it. The shared gating `PersistentPreRunE` is cobra code too, so it stays in `cmd/`. The command's logic in `internal/cli/` knows nothing about contexts.

**Why.** The boundary is checkable by lint, so "commands hold no business logic" stops being a review convention, and so does "cobra stays in one place": swapping or wrapping the framework only touches `cmd/`. Command logic is testable with plain table tests (no command, no `Execute`, no `t.Setenv`), and `cmd/` needs only a few wiring tests. The cost is that a command's help text and flags sit in `cmd/`, away from its logic, and that flags are declared in `cmd/` and passed on as plain values.

**Rule of thumb.** Anything worth a unit test goes in `internal/cli/<cmd>/` (or a domain package). Help text is not logic: it lives in `cmd/` beside `Use` and the flags.

Package names beyond these are decided with the code that needs them. A package is named for what it provides, not for a layer. No `util`, `common` or `helpers`.

### Tasks: single actions that compose

**Status: `pushbranch`, `createpr` and `createbranch` are built and `ship` uses them. The other tasks are planned.** `VISION.md` asks for the modular, "unix-style" composability of the old scripts. The idiomatic Go form of a pipe is small typed functions called in order by ordinary code, the way `io.Reader` and `io.Writer` compose, not a stream of bytes between processes. So a **task** is one package under `internal/task/<name>/` holding one action that is useful alone and as a step:

* `createbranch`: a new branch for a project and release line, based on the upstream branch, made in the engineer's clone or in a temporary worktree of it, or an earlier branch of ours reused when it is current.
* `bumpcommit`: apply one change set and make its commits. The change and its commits are never separated.
* `pushbranch`: push a branch.
* `createpr`: open the PR for a branch that is pushed, with the labels the repository has. Closing the PRs it supersedes is not part of it: `ship` also does that when a PR is already open and nothing is created, so it is a step of its own, not built as a task yet.
* More are added when a second caller needs them, not before.

**Shape.** Every task has the same three parts, and nothing else is exported that a caller must know about:

* An `Input` of plain values: strings, bools, small structs. No cobra, no `Settings`, no config types the task does not read.
* An `Output` of plain, serializable values: what the next step needs and what a report shows (the branch name, the commits made, the PR URL). The `Output` of one task is meant to be the `Input` of the next, and is what `--json` prints for a task run on its own. The one exception is a branch being worked on: `createbranch` returns a `*workdir.Work`, a handle that the tasks which commit to it and end the work take back, from `internal/workdir`. It is shared, not serialized, so that pair is chained in Go only.
* A `Run(ctx, deps, in) (Output, error)` with `deps` a small interface the task defines for what it calls, so a task is tested with a hand-written fake of that interface and nothing wider ([TESTING](TESTING.md)). A task reports progress through an optional event sink and never prints.

**Who composes.** A task never calls another task. `internal/cli/<cmd>/` does, in two shapes. A *gadget* command (`gero branch create`, `gero pr create`) builds the deps, calls one task and renders its `Output`. A *conveyor* command (`gero ship`) calls several in order, passing each `Output` to the next `Input`, and owns what the tasks must not: the loops over targets, the decision whether to run a step, the one confirmation question and the final report. A conveyor's logic that is worth a test stays in its own domain package (`internal/ship`); the tasks are what it calls.

**Pipes between commands.** Chaining in Go is the pipe. Chaining at the shell, one `gero` run reading what the last printed, needs nothing new in the layout: a task's `--json` output is its `Output`, and a later flag that reads an `Input` from a file or stdin would complete it. That flag is not designed. `gero repl` has no pipes ([REPL](REPL.md#boundaries)), and gadgets reach it as commands in the same tree.

**Why.** `ship` already does these steps, but through an unexported `resolution` and a `Publisher` and `Executor` that carry the whole configuration, and through `ship.Git` and `ship.Remote`, which are wide interfaces for a test to fake. Only `Prepare`, `Apply.Run` and the `Work` lifecycle in `internal/ship` are exported and usable on their own today, and `Apply.Run` still needs a `*Work` and the update dispatcher's invocation. Splitting by action keeps each interface to what one action calls, lets an engineer run a part of a process without the whole of it, and keeps testing and coverage per action.

## 3. Dependency direction

```
main.go -> cmd -> internal/cli/<cmd>  ->  domain packages  ->  internal/config, internal/ui, ...
                         \                                          ^
                          `--> internal/clicfg  (Settings)          |
                                                                    |
                          leaf packages import nothing from internal/
```

* `cmd/` imports only the standard library, cobra, `cmd/...`, `internal/cli/...`, `internal/clicfg` and `internal/errs`. `main.go` imports only `cmd`. Nothing imports `cmd/`.
* `internal/cli/<cmd>/` is the only layer that calls into domain packages on behalf of a command. It never imports cobra. It may import any other `internal/` package, but not another command package. Shared command code goes in a third, smaller package.
* `internal/task/<name>/` imports the standard library and domain packages, never `internal/cli/...`, cobra, `internal/ui` or another task (lint-enforced). Tasks are composed only by `internal/cli/<cmd>/` and by a conveyor's domain package such as `internal/ship`, which is by review.
* Domain packages (`semver`, `git`, `config`, ...) never import `internal/cli/...` or cobra. Cobra is confined to `cmd/`, so neither the command logic nor a domain package knows anything about the CLI framework.
* `internal/clicfg` and `internal/config` (with its `loader`) do not import each other. `Settings` is passed down by the command, never read from a global.
* `internal/workspace` is where the two meet. It imports `clicfg`, `config`, `config/loader` and `internal/git/repo`, and nothing under `cmd/` or `internal/cli/`. Command packages reach central config and repo resolution only through it, by calling `workspace.Open(settings, cwd)` with plain values. `cmd/` does not import it, so `cmd/` stays pure translation.
* Domain packages take their dependencies as arguments (a `*slog.Logger`, a GitHub client interface, a `Settings` value). No package-level logger, client or config singleton.
* Interfaces are defined where they are consumed, so a domain package can be tested without the real GitHub client.
* No import cycles, and no cross-domain imports without a reason recorded here. If two domain packages need each other, the shared part is a third, smaller package.

## 4. Where code goes

| Code | Home |
|---|---|
| Root command, global flags, registering commands | `cmd/` |
| Command tree, `Use` and help text, flag declarations, gating, the `RunE` glue (args, flags and env to plain values, then output) | `cmd/` |
| What a command does with those values (resolving inputs, rendering output), as plain functions | `internal/cli/<cmd>/` |
| Anything with a branch worth testing beyond input handling and rendering | a domain package in `internal/`, never in `cmd/` |
| One action that a command may run alone or a higher-order command may chain | `internal/task/<name>/` |
| Test helpers | `_test.go` files, or a `internal/<pkg>/<pkg>test` package if several packages share them |
| Golden files and fixtures | `testdata/` beside the package that uses them |

## 5. Integration tests live in a sub-module

Integration tests may need heavy dependencies (a container runner, a fake GitHub server, a Kubernetes test environment). Those must not enter the root `go.mod`, which `gero` users and the supply-chain audit see ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)).

* `test/integration/` has its own `go.mod` (module `github.com/rancher/orbs-automation/test/integration`). Its dependencies, and their transitive tree, stay out of the root module. `go build ./...` and `go test ./...` at the root do not see it.
* **Black box by default.** The tests build and run the `gero` binary and assert on stdout, stderr, exit code and side effects. They do not import `internal/`, so the sub-module needs no `replace` directive on the root and no `go.work`. If a test truly needs to import root packages, record the reason here and add a `replace ../..` in the sub-module's `go.mod`, not a `go.work` checked in.
* Unit tests stay beside their package and use only the root module's dependencies. The sub-module is for tests that need processes, the network or containers.
* The sub-module is run by its own CI job (`cd test/integration && go test ./...`), separate from the fast unit job, and is not part of the pre-commit loop.
* It is linted and vetted separately (`golangci-lint run` from inside it), with its own `.golangci.yml` or a shared config. Its dependencies are recorded in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md), since the root depguard allow-list does not cover it.

## 6. Enforcement

* The `internal/` rule is compiler-enforced.
* Imports of third-party modules are checked by the depguard allow-list ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)).
* **The boundaries below are lint-enforced** by depguard rules in [`.golangci.yml`](../../.golangci.yml), each with a comment saying why. The config is the source of truth for the exact lists; this is the intent.
  * `cmd/` imports only the standard library, cobra, its own subpackages, `internal/cli/...`, `internal/clicfg` and `internal/errs`. `main.go` imports only `cmd`.
  * cobra is importable only from `cmd/` and `main.go`.
  * Each outside dependency has one door: go-git only in `internal/git/repo`, `os/exec` only in `internal/execx`, the YAML library only in `internal/clicfg` and `internal/config/loader`, go-gh only in `internal/github/gogh.go`, go-github only in `internal/github`.
  * `internal/cli/...` reaches the repository on disk only through `internal/workspace`, never `internal/git/repo` directly. Its pure logic may still take `internal/git` and domain types.
  * `internal/task/**` imports neither `internal/cli`, `internal/ui` nor another task.
* **Central config has one door.** `internal/config/loader` is importable only from `internal/workspace` (lint-enforced). A command package cannot load config, apply defaults or skip validation: it gets a `config.Project` or `config.Config` from `Workspace`, already defaulted and validated, and imports `internal/config` only for the types and their getters.
* **By review, for now:** the rest of dependency direction above, including that no command package imports another. A depguard rule per package can be added once there are enough packages to be worth it.

## Why `clicfg` is not under `internal/cli/`

`internal/clicfg` is a cobra-free leaf that other packages may use, while `internal/cli/` is one package per command. How flags reach it is in [CLI_CONFIG](CLI_CONFIG.md).

## Not built yet

* The prompt side of `internal/ui` ([CLI_OUTPUT](CLI_OUTPUT.md)). `Output` and `Stream` are built.
* The other tasks, `bumpcommit` and a task for closing superseded PRs, extracted from `internal/ship` one action at a time, and the standalone commands that run a task alone. `pushbranch`, `createpr` and `createbranch` are built, and have no command of their own yet.
* `test/integration/` ([TESTING](TESTING.md)).

## Open questions

* Whether generated code (for example JSON schema from config types) gets its own directory.
