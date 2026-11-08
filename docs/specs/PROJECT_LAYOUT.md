# Project Layout

**Status:** implemented, except three of the standalone commands for the tasks, a task for closing superseded PRs, and `test/integration/` (see Not built yet).

How Go code is organized in this repo. `gero` is a CLI, not a library, so the default is that nothing is importable from outside the module.

## 1. The `internal/` rule

* All Go packages live under `internal/`, except the `main` package (`main.go` at the repo root) and the cobra layer under `cmd/`. The compiler enforces that code outside this module cannot import `internal/`.
* There is no `pkg/` directory. Moving a package out of `internal/` later is cheap. Retracting a public API is not.
* **Promotion criteria.** A package leaves `internal/` only when all hold: a real external consumer exists, its API has survived at least one minor release unchanged, and it has no dependency on other `internal/` packages that are not themselves promoted. Promotion is a decision recorded in this spec, not a drive-by move.
* Data files (`config/`, `mock-repos/`) are not Go code and stay at the top level.

## 2. Layout

```
main.go                entry point: calls cmd.Execute(), nothing else
cmd/                   all cobra code: root command, command tree, flags, help text, groups, gating; imports only internal/cli, internal/platform/clicfg and internal/platform/errs
cmd/version/           build info (Version, Commit, Date) set by GoReleaser through -ldflags -X; read only by cmd/, passed down as a value

internal/cli/<cmd>/    one subpackage per command: its logic as plain functions, no cobra
internal/cli/logging/  shared, not a command: builds the *slog.Logger, called once by cmd/   (CLI_OUTPUT)
internal/cli/termfacts/  shared, not a command: the TTY and width checks of the process, handed to clicfg.Load; the only importer of golang.org/x/term
internal/cli/identity/, cdpath/, cigate/, buildinfo/  shared, not commands: who commits are attributed to, the logic of gero cd, whether a CI-only command may run, the build's version
internal/ui/           the output seam: Output, Stream and Align (columns laid out by what a cell shows)   (CLI_OUTPUT)
internal/ui/style/     the one importer of lipgloss: named styles and the Renderer built from clicfg.Terminal; plain when color is off   (CLI_OUTPUT)
internal/ui/editor/    the one importer of ergochat/readline: the line reader seam, with a plain bufio implementation, for gero repl   (REPL)

internal/workspace/    the one door from Settings to the world: config dir, the joined config and the update engine, repo paths, upstream, fork   (WORKSPACE)
internal/ship/         one pull request for each target branch of a consumer: the state check and the write sequence, with its interfaces   (PUSH_TO_REPO). It still holds most of the actions; they move to internal/task/ and ship keeps the decisions and the order
internal/task/<name>/  one package per single action (create a branch, bump and commit, push a branch, open a PR, make a tag), run by a command alone or chained by a higher-order one such as ship   (see Tasks below); pushbranch, createpr, createbranch, bumpcommit and createtag are built
internal/bump/         the update engine: the routes of release holdings, the targets of a release, writing a holding in place, the edits' strategies and steps around the write; starts no process   (UPDATE_ENGINE)
internal/steprun/      runs the pre-edit and post-edit steps of an update
internal/registry/     image existence checks in the registries a project publishes to: the registries gero uses and the report over a project's image artifacts   (IMAGES_EXIST)
internal/tagscheme/    what readers ask of a project's tags: its lines, its newest release, the order of two tags
internal/calver/       the tag format of projects whose lines are in their tags, such as rancher-assets
internal/releasetag/   the policy of a release line's tags: what its newest tag is, what it could have next, which tags it may not have, and the line's state; pure, asked by the tag task, `repo tag` and `repo status`
internal/lifecycle/    reads Rancher's lifecycle dates from the SUSE lifecycle page; the only importer of golang.org/x/net/html
internal/...           domain packages as they arrive, such as the reconciler's

internal/apis/meta/    what every version of the config API shares: the envelope of a document (apiVersion, kind, name, refs) and the Scheme that decodes a YAML stream into the kinds each version registers
internal/apis/lines/    what every version of the config API says of a Rancher line: its id, a range of lines, a line's dates, phase and Kubernetes range
internal/apis/v1alpha2/  the kinds of apiVersion gero/v1alpha2: their types, YAML form, defaults and rules, and the rules across a set of documents   (docs/design/CONFIG_SCHEMA.md)
internal/appconfig/    application config provider of gero/v1alpha2: reads every YAML file of a directory, validates each document and the set
internal/chain/        the resolved model commands read, and the only adapter from an API version to the domain (the `oneversion` lint rule keeps v1alpha2 to appconfig, chain and chainload): projects, streams, artifacts, holdings and edits joined, and the answers across them   (CHAIN_MODEL)
internal/chainload/    reads a directory into the chain's graph (workspace only)

internal/platform/actor/      who commits are attributed to: the engineer's git identity, or the bot's in CI   (EXECUTION_CONTEXT)
internal/platform/ociregistry/ asks an image registry whether an image exists: the manifest request and the anonymous token, no knowledge of which registries gero uses
internal/platform/clicfg/     the user file (schema.go, unversioned: not application config) and the CLI config loader -> Settings   (CLI_CONFIG)
internal/platform/config/     the config engine: decodes one object strictly, fills its defaults, validates it, and reports every finding, errors and warnings   (docs/design/CONFIG_ENGINE.md)
internal/platform/execx/      the one place gero starts another program: allow-listed binaries, raw stdout, stderr and exit code, no formatting; the only importer of os/exec
internal/platform/errs/       error values shared across gero, such as NotImplemented
internal/platform/semver/     release tag classification, as the check-semver script did, and a Series of versions (9.x, 0.9.x) that holds versions and overlaps another   (CHECK_SEMVER)
internal/platform/git/        pure logic about git refs, tags and commits (branch tags, commit abbreviation); no go-git, no exec
internal/platform/git/repo/   local repository access with go-git; the only importer of go-git
internal/platform/git/gitcli/ escape hatches to the git binary, one documented function per thing go-git cannot do; runs git through execx
internal/platform/github/     gero's access to GitHub: the host and token, the REST client, the rate limit, raw file reads
internal/platform/github/pulls/  the PR service: find, create, comment on, close and label pull requests, in plain types; where go-github's pull request calls stay
internal/platform/ghcli/      finds and runs the gh binary for gero doctor (version only); runs gh through execx
internal/platform/workdir/    the branch a run edits and commits in: made in the engineer's clone or a temporary worktree of it, committed to, and put back when the work ends or fails; shared by the tasks that make a branch and the ones that commit to it
internal/platform/templates/  the embedded commit, body and title templates and their placeholder renderer
internal/platform/yamledit/   changes one YAML value in place without reflowing the file
internal/platform/shellenv/   the shell, its profile, the script and the marked block gero adds   (SHELL_SETUP)

test/integration/      separate Go module (own go.mod) for integration tests     (TESTING)
```

**Decision: cobra lives only in `cmd/`, command logic lives in `internal/cli/`.** `cmd/` (package `cmd`, with `main.go` at the repo root, as `cobra-cli init` generates) is the one place that knows about cobra. It holds the root command, the global flags, the command tree, each command's `Use`, help text, flags and annotations, the help groups, the context gating, and a short `RunE` per command. That `RunE` only translates: it turns args, flags and the environment into plain values, calls a function in `internal/cli/<cmd>/`, writes the result to `cmd.OutOrStdout()` and returns the error. It may import the standard library, cobra, its own subpackages (`cmd/...`), `internal/cli/...`, `internal/platform/clicfg` and `internal/platform/errs` (exit-code errors, needed to wire `Execute`). It may **not** import any other `internal/` package. Its use of `internal/platform/clicfg` is narrow: it builds a `clicfg.FlagLookup` over the parsed flags, calls `clicfg.Load` once in the root `PersistentPreRunE`, and passes the resulting `Settings` down by value.

`internal/cli/<cmd>/` holds everything a command does beyond that translation, as plain exported functions that never import cobra. Inputs are plain values: strings, an options struct for flags, and a `getenv func(string) string` instead of `os.Getenv`. Outputs are returned values (a string, a struct) or an `io.Writer` argument, plus errors, with sentinel errors where a caller needs to tell them apart. Example: `gero semver check` is `checksemver.ResolveTag(args, getenv) (string, error)`, `checksemver.Classify(tag) semver.Info` and `checksemver.Render(info) string`, called from the `RunE` in `cmd/repo.go`. A noun command such as `gero repo` is a parent in `cmd/`; each child (`gero semver check`, `gero repo build-tags`) keeps its own `internal/cli/<child>/` package, named for its logic (`checksemver`, `branchtags`).
**CI-only commands.** The help group is still a `cobra.Group` ("CI-Only Commands:") following `ob-charts-tool`. Its definition and the `gero/context` annotation constants live in `cmd/groups/`. A CI-only command prefixes its `Use` with the group ID (`gero ci:reconcile`) and sets the annotation and `GroupID` where `cmd/` defines it. The shared gating `PersistentPreRunE` is cobra code too, so it stays in `cmd/`. The command's logic in `internal/cli/` knows nothing about contexts.

**Why.** The boundary is checkable by lint, so "commands hold no business logic" stops being a review convention, and so does "cobra stays in one place": swapping or wrapping the framework only touches `cmd/`. Command logic is testable with plain table tests (no command, no `Execute`, no `t.Setenv`), and `cmd/` needs only a few wiring tests. The cost is that a command's help text and flags sit in `cmd/`, away from its logic, and that flags are declared in `cmd/` and passed on as plain values.

**Rule of thumb.** Anything worth a unit test goes in `internal/cli/<cmd>/` (or a domain package). Help text is not logic: it lives in `cmd/` beside `Use` and the flags.

Package names beyond these are decided with the code that needs them. A package is named for what it provides, not for a layer. No `util`, `common` or `helpers`.

### Tasks: single actions that compose

**Status: `pushbranch`, `createpr`, `createbranch`, `bumpcommit` and `createtag` are built. `ship` uses the first four, and `createbranch` and `createtag` each have a command that runs them alone (`repo branch`, `repo tag`). Closing superseded PRs is not a task.** `VISION.md` asks for the modular, "unix-style" composability of the old scripts. The idiomatic Go form of a pipe is small typed functions called in order by ordinary code, the way `io.Reader` and `io.Writer` compose, not a stream of bytes between processes. So a **task** is one package under `internal/task/<name>/` holding one action that is useful alone and as a step:

* `createbranch`: a new branch for a project and release line, based on the upstream branch, made in the engineer's clone or in a temporary worktree of it, or an earlier branch of ours reused when it is current.
* `bumpcommit`: apply one change set and make its commits. The change and its commits are never separated.
* `createtag`: make a lightweight tag at a commit in the engineer's clone, after the line's rules for its tags ([`internal/releasetag`](../../internal/releasetag/releasetag.go)) have said it may be made. It never moves a tag, and sending it is a push.
* `pushbranch`: push a branch.
* `createpr`: open the PR for a branch that is pushed, with the labels the repository has. Closing the PRs it supersedes is not part of it: `ship` also does that when a PR is already open and nothing is created, so it is a step of its own, not built as a task yet.
* More are added when a second caller needs them, not before.

**Shape.** Every task has the same three parts, and nothing else is exported that a caller must know about:

* An `Input` of plain values: strings, bools, small structs. No cobra, no `Settings`, no config types the task does not read.
* An `Output` of plain, serializable values: what the next step needs and what a report shows (the branch name, the commits made, the PR URL). The `Output` of one task is meant to be the `Input` of the next, and is what `--json` prints for a task run on its own. The one exception is a branch being worked on: `createbranch` returns a `*workdir.Work` from `internal/platform/workdir`, and `bumpcommit` takes it back in its `Input`, with the caller ending the work. It is a handle, shared and not serialized, so that pair is chained in Go only.
* A `Run(ctx, deps, in) (Output, error)` with `deps` a small interface the task defines for what it calls, so a task is tested with a hand-written fake of that interface and nothing wider ([TESTING](TESTING.md)). A task reports progress through an optional event sink and never prints.

**Who composes.** A task never calls another task. `internal/cli/<cmd>/` does, in two shapes. A *gadget* command (`gero repo branch`, `gero repo tag`, `gero pr create`; [TASK_COMMANDS](TASK_COMMANDS.md)) builds the deps, calls one task and renders its `Output`. A *conveyor* command (`gero ship`) calls several in order, passing each `Output` to the next `Input`, and owns what the tasks must not: the loops over targets, the decision whether to run a step, the one confirmation question and the final report. A conveyor's logic that is worth a test stays in its own domain package (`internal/ship`); the tasks are what it calls.

**Pipes between commands.** Chaining in Go is the pipe. Chaining at the shell, one `gero` run reading what the last printed, needs nothing new in the layout: a task's `--json` output is its `Output`, and a later flag that reads an `Input` from a file or stdin would complete it. That flag is not designed. `gero repl` has no pipes ([REPL](REPL.md#boundaries)), and gadgets reach it as commands in the same tree.

**Why.** `ship` already does these steps, but through an unexported `resolution` and a `Publisher` and `Executor` that carry the whole configuration, and through `ship.Git` and `ship.Remote`, which are wide interfaces for a test to fake. Only `Prepare`, `Apply.Run` and the `Work` lifecycle in `internal/ship` are exported and usable on their own today, and `Apply.Run` still needs a `*Work` and the update dispatcher's invocation. Splitting by action keeps each interface to what one action calls, lets an engineer run a part of a process without the whole of it, and keeps testing and coverage per action.

## 3. Dependency direction

`internal/` has horizontal layers. Imports point down and never up:

```
main.go -> cmd
cmd     -> internal/cli, internal/ui         (commands and the output seam)
cli, ui -> domain packages
domain  -> internal/apis
apis    -> internal/platform
```

| Layer | Holds | May import |
|---|---|---|
| `internal/platform/` | low-level packages that know nothing of projects, releases or lines: clients, exec, YAML editing, version parsing, templates, the CLI settings loader | other platform packages and third-party libraries |
| `internal/apis/` | the versioned config contract: the envelope and the kinds of each apiVersion | `internal/platform` and `internal/apis` |
| domain (`ship`, `workspace`, `bump`, `chain`, `chainload`, `appconfig`, `tagscheme`, `calver`, `releasetag`, `registry`, `steprun`, `task/*`, `lifecycle`) | what gero knows about projects, releases and shipping | platform, apis and domain |
| `internal/cli/`, `internal/ui/` | one package per command, and the output seam | everything below them |
| `cmd/` | the cobra shell | `internal/cli` and the two platform packages named below |

* `cmd/` imports only the standard library, cobra, `cmd/...`, `internal/cli/...`, `internal/platform/clicfg` and `internal/platform/errs`. `main.go` imports only `cmd`. Nothing imports `cmd/`.
* `internal/cli/<cmd>/` is the only layer that calls into domain packages on behalf of a command. It never imports cobra. It may import any lower package, but not another command package, with two exceptions: shared packages that are not commands (`identity`, `cdpath`, `termfacts`, `cigate`, `logging`, `buildinfo`), and `doctor`, which reuses `authstatus` and `configresolve` for its checks. Other shared command code goes in a third, smaller package.
* `internal/task/<name>/` imports the standard library and lower packages, never `internal/cli/...`, cobra, `internal/ui` or another task. Tasks are composed only by `internal/cli/<cmd>/` and by a conveyor's domain package such as `internal/ship`, which is by review.
* Domain packages never import `internal/cli/...`, `internal/ui` or cobra. Cobra is confined to `cmd/`, so neither the command logic nor a domain package knows anything about the CLI framework.
* `internal/platform/clicfg` and the config packages (`internal/chain` and the ones that build it) do not import each other. `Settings` is passed down by the command, never read from a global.
* `internal/workspace` is where the two meet. It imports `clicfg`, `chain`, `chainload`, `bump` and `internal/platform/git/repo`, and nothing under `cmd/` or `internal/cli/`. Command packages reach central config and repo resolution only through it, by calling `workspace.Open(settings, cwd)` with plain values. `cmd/` does not import it, so `cmd/` stays pure translation.
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
* **The boundaries below are lint-enforced** by depguard rules in [`.golangci.yml`](../../.golangci.yml), each with a comment saying why. The config is the source of truth for the exact lists; this is the intent. Each rule is written as a deny with an allow carve-out, so a new package never needs a rule edit and a stale path cannot quietly stop denying.
  * **The layers:** platform imports only platform; apis imports only platform and apis; domain packages do not import `internal/cli` or `internal/ui`; a command does not import another command except the shared helpers (and `doctor` its two named ones).
  * `cmd/` imports only the standard library, cobra, its own subpackages, `internal/cli/...`, `internal/platform/clicfg` and `internal/platform/errs`. `main.go` imports only `cmd`.
  * cobra is importable only from `cmd/` and `main.go`.
  * Each outside dependency has one door: go-git only in `internal/platform/git/repo`, `os/exec` only in `internal/platform/execx`, the YAML library only in `internal/platform/config`, `internal/apis` and `internal/platform/yamledit`, the HTML parser only in `internal/lifecycle`, lipgloss only in `internal/ui/style`, the line editor only in `internal/ui/editor`, `golang.org/x/term` only in `internal/cli/termfacts`, go-gh only in `internal/platform/github/gogh.go`, go-github only in `internal/platform/github`.
  * `internal/cli/...` reaches the repository on disk only through `internal/workspace`, never `internal/platform/git/repo` directly. Its pure logic may still take `internal/platform/git` and domain types.
  * `internal/task/**` imports neither `internal/cli`, `internal/ui` nor another task.
* **Central config has one door.** `internal/chainload` and `internal/appconfig` are importable only from `internal/workspace` (lint-enforced; `internal/chain` also imports the provider's type). A command package cannot load config, apply defaults or skip validation: it gets a `*chain.Graph` or a `*chain.Project` from `Workspace`, already defaulted and validated, and imports `internal/chain` for the types and their answers.
* **By review:** that tasks are composed only by commands and a conveyor's domain package, and that a package is named for what it provides.

## Why `clicfg` is under `internal/platform/` and not `internal/cli/`

`internal/platform/clicfg` is a cobra-free leaf that other packages use, while `internal/cli/` is one package per command. How flags reach it is in [CLI_CONFIG](CLI_CONFIG.md).

## Not built yet

* A shared yes/no confirmation in `internal/ui`: `ship`, `repo tag` and `shell install` each ask their own `[y/N]` through the style renderer ([CLI_OUTPUT](CLI_OUTPUT.md)).
* A task for closing superseded PRs, extracted from `internal/ship` when it is needed, and the standalone commands for `pushbranch`, `createpr` and `bumpcommit` ([TASK_COMMANDS](TASK_COMMANDS.md)). `gero repo branch` and `gero repo tag` run `createbranch` and `createtag` alone.
* Narrowing `ship.Git` and `ship.Remote` now that the tasks define what they call: both still carry every method any task needs.
* `test/integration/` ([TESTING](TESTING.md)).

## Open questions

* Whether generated code (for example JSON schema from config types) gets its own directory.
