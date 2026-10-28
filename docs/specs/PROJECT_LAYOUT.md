# Project Layout

**Status:** agreed. Nothing is built yet.

How Go code is organized in this repo. `gero` is a CLI, not a library, so the default is that nothing is importable from outside the module.

## 1. The `internal/` rule

* All Go packages live under `internal/`, except the `main` package (`main.go` at the repo root) and the cobra layer under `cmd/`. The compiler enforces that code outside this module cannot import `internal/`.
* There is no `pkg/` directory. Moving a package out of `internal/` later is cheap. Retracting a public API is not.
* **Promotion criteria.** A package leaves `internal/` only when all hold: a real external consumer exists, its API has survived at least one minor release unchanged, and it has no dependency on other `internal/` packages that are not themselves promoted. Promotion is a decision recorded in this spec, not a drive-by move.
* Data files (`config/`, `config-mock/`, `mock-repos/`) are not Go code and stay at the top level.

## 2. Layout

```
main.go              entry point: calls cmd.Execute(), nothing else
cmd/                 all cobra code: root command, command tree, flags, help text, groups, gating; imports only internal/cli and internal/errs
internal/cli/<cmd>/  one subpackage per command: its logic as plain functions, no cobra
internal/clicfg/     CLI config loader -> Settings            (CLI_CONFIG)
internal/config/     application config: types, loader, validation, defaults
internal/ui/         output and prompt seam, plain-text implementation first   (CLI_OUTPUT)
internal/...         domain packages as they arrive: lines, bump, reconcile, github
test/integration/    separate Go module (own go.mod) for integration tests     (TESTING)
```

**Decision: cobra lives only in `cmd/`, command logic lives in `internal/cli/`.** `cmd/` (package `cmd`, with `main.go` at the repo root, as `cobra-cli init` generates) is the one place that knows about cobra. It holds the root command, the global flags, the command tree, each command's `Use`, help text, flags and annotations, the help groups, the context gating, and a short `RunE` per command. That `RunE` only translates: it turns args, flags and the environment into plain values, calls a function in `internal/cli/<cmd>/`, writes the result to `cmd.OutOrStdout()` and returns the error. It may import the standard library, cobra, its own subpackages (`cmd/...`), `internal/cli/...` and `internal/errs` (exit-code errors, needed to wire `Execute`). It may **not** import any other `internal/` package.

`internal/cli/<cmd>/` holds everything a command does beyond that translation, as plain exported functions that never import cobra. Inputs are plain values: strings, an options struct for flags, and a `getenv func(string) string` instead of `os.Getenv`. Outputs are returned values (a string, a struct) or an `io.Writer` argument, plus errors, with sentinel errors where a caller needs to tell them apart. Example: `gero repo check-semver` is `checksemver.ResolveTag(args, getenv) (string, error)`, `checksemver.Classify(tag) semver.Info` and `checksemver.Render(info) string`, called from the `RunE` in `cmd/repo.go`. A noun command such as `gero repo` is a parent in `cmd/`; each child (`check-semver`, `branch-tags`) keeps its own `internal/cli/<child>/` package. This replaces the earlier plan where each package exported a `*cobra.Command` constructor, and the plans before that (command code under `cmd/<group>/`, and `main` under `cmd/gero/`).

**CI-only commands.** The help group is still a `cobra.Group` ("CI-Only Commands:") following `ob-charts-tool`. Its definition and the `gero/context` annotation constants live in `cmd/groups/`. A CI-only command prefixes its `Use` with the group ID (`gero ci:reconcile`) and sets the annotation and `GroupID` where `cmd/` defines it. The shared gating `PersistentPreRunE` is cobra code too, so it stays in `cmd/`. The command's logic in `internal/cli/` knows nothing about contexts.

**Why.** The boundary is checkable by lint, so "commands hold no business logic" stops being a review convention, and so does "cobra stays in one place": swapping or wrapping the framework only touches `cmd/`. Command logic is testable with plain table tests (no command, no `Execute`, no `t.Setenv`), and `cmd/` needs only a few wiring tests. The cost is that a command's help text and flags sit in `cmd/`, away from its logic, and that flags are declared in `cmd/` and passed on as plain values.

**Rule of thumb.** Anything worth a unit test goes in `internal/cli/<cmd>/` (or a domain package). Help text is not logic: it lives in `cmd/` beside `Use` and the flags.

Package names beyond these are decided with the code that needs them. A package is named for what it provides, not for a layer. No `util`, `common` or `helpers`.

## 3. Dependency direction

```
main.go -> cmd -> internal/cli/<cmd>  ->  domain packages  ->  internal/config, internal/ui, ...
                         \                                          ^
                          `--> internal/clicfg  (Settings)          |
                                                                    |
                          leaf packages import nothing from internal/
```

* `cmd/` imports only the standard library, cobra, `cmd/...`, `internal/cli/...` and `internal/errs`. `main.go` imports only `cmd`. Nothing imports `cmd/`.
* `internal/cli/<cmd>/` is the only layer that calls into domain packages on behalf of a command. It never imports cobra. It may import any other `internal/` package, but not another command package. Shared command code goes in a third, smaller package.
* Domain packages (`semver`, `gittags`, `config`, ...) never import `internal/cli/...` or cobra. Cobra is confined to `cmd/`, so neither the command logic nor a domain package knows anything about the CLI framework.
* `internal/clicfg` and `internal/config` do not import each other. `Settings` is passed down by the command, never read from a global.
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
* **The `cmd/` boundary is lint-enforced.** A second depguard rule scoped to `cmd/` (`files: ["**/cmd/**"]`, strict) allows only `$gostd`, `github.com/spf13/cobra`, `github.com/rancher/orbs-automation/cmd`, `.../internal/cli` and `.../internal/errs` (depguard matches by prefix, so subpackages are covered). A matching rule for `main.go` allows only `$gostd` and `github.com/rancher/orbs-automation/cmd`. A third rule, `nocobra`, denies `github.com/spf13/cobra` in every file outside `cmd/`, `main.go` and tests, so `internal/cli/` and the domain packages cannot import it. The rest of dependency direction (section 3) is by review for now. A depguard rule per package can be added once there are enough packages to be worth it.

## Open questions

* How flags reach `internal/cli/<cmd>/` and `internal/clicfg` without cobra. **(leaning)** `cmd/` declares the flags and hands over a framework-neutral structure (the values, and which flags were explicitly set), defined in `internal`. This overlaps with how every setting is resolved from flags, environment, user file and defaults, which is why viper was not paired with cobra ([CLI_CONFIG](CLI_CONFIG.md) section 2). So it is designed together with the `internal/clicfg` loader and `Settings`, not per command. Until the first command with flags, a command takes plain arguments.
* Whether `internal/github` wraps `go-gh` and `go-github` behind one interface, or exposes two. Decided with the GitHub access spike (IMPLEMENTATION §3).
* Whether generated code (for example JSON schema from config types) gets its own directory.
