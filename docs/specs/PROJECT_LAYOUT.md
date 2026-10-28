# Project Layout

**Status:** agreed. Nothing is built yet.

How Go code is organized in this repo. `gero` is a CLI, not a library, so the default is that nothing is importable from outside the module.

## 1. The `internal/` rule

* All Go packages live under `internal/`, except the `main` package (`main.go` at the repo root) and the thin cobra entry point under `cmd/`. The compiler enforces that code outside this module cannot import `internal/`.
* There is no `pkg/` directory. Moving a package out of `internal/` later is cheap. Retracting a public API is not.
* **Promotion criteria.** A package leaves `internal/` only when all hold: a real external consumer exists, its API has survived at least one minor release unchanged, and it has no dependency on other `internal/` packages that are not themselves promoted. Promotion is a decision recorded in this spec, not a drive-by move.
* Data files (`config/`, `config-mock/`, `mock-repos/`) are not Go code and stay at the top level.

## 2. Layout

```
main.go              entry point: calls cmd.Execute(), nothing else
cmd/                 root command, global flags, registration of commands; imports only internal/cli and internal/errs
internal/cli/<cmd>/  one subpackage per command: cobra command, flags, RunE, help text
internal/cli/groups/ cobra help groups and the context annotation shared by commands
internal/clicfg/     CLI config loader -> Settings            (CLI_CONFIG)
internal/config/     application config: types, loader, validation, defaults
internal/ui/         output and prompt seam, plain-text implementation first   (CLI_OUTPUT)
internal/...         domain packages as they arrive: lines, bump, reconcile, github
test/integration/    separate Go module (own go.mod) for integration tests     (TESTING)
```

**Decision: `cmd/` is a thin shell, commands live in `internal/cli/`.** `cmd/` (package `cmd`, with `main.go` at the repo root, as `cobra-cli init` generates) holds only the root command, the global flags, and the registration of commands. It may import the standard library, public (third-party) packages from the depguard allow-list, its own subpackages (`cmd/...`), `internal/cli/...` and `internal/errs` (exit-code errors, needed to wire `Execute`). It may **not** import any other `internal/` package. Each command is its own subpackage, `internal/cli/<cmd>/`, exporting a constructor (for example `checksemver.New() *cobra.Command`) that `cmd/` registers. A noun command such as `gero repo` is a parent whose children are registered under it in `cmd/`; each child (`check-semver`, `branch-tags`) keeps its own package. This replaces the earlier plan of command code under `cmd/` and `cmd/<group>/`, and, before that, `main` under `cmd/gero/`.

**CI-only commands.** The help group is still a `cobra.Group` ("CI-Only Commands:") following `ob-charts-tool`. Its definition and the `gero/context` annotation constants move to `internal/cli/groups/`, which command packages import. A CI-only command prefixes its `Use` with the group ID (`gero ci:reconcile`) and sets the annotation and `GroupID` in its own constructor, so a command package is self-describing and `cmd/` does no per-command setup beyond `AddCommand`. **(leaning)** If the shared gating `PersistentPreRunE` needs a home, it also goes in `internal/cli/groups/`.

**Why.** The boundary is checkable by lint, so "commands hold no business logic" stops being a review convention. Command packages are testable without the root command (build the `*cobra.Command`, set args and an output buffer, execute). And a command's flags, help text and wiring sit next to each other instead of being spread over `cmd/`.

Package names beyond these are decided with the code that needs them. A package is named for what it provides, not for a layer. No `util`, `common` or `helpers`.

## 3. Dependency direction

```
main.go -> cmd -> internal/cli/<cmd>  ->  domain packages  ->  internal/config, internal/ui, ...
                         \                                          ^
                          `--> internal/clicfg  (Settings)          |
                                                                    |
                          leaf packages import nothing from internal/
```

* `cmd/` imports only the standard library, allow-listed public packages, `cmd/...`, `internal/cli/...` and `internal/errs`. `main.go` imports only `cmd`. Nothing imports `cmd/`.
* `internal/cli/<cmd>/` is the only layer that imports cobra-facing glue and calls into domain packages. It may import any other `internal/` package, but not another command package. Shared command code goes in `internal/cli/groups/` or a third, smaller package.
* Domain packages (`semver`, `gittags`, `config`, ...) never import `internal/cli/...` or cobra. Cobra is confined to the CLI layer (`cmd/` and `internal/cli/`), so a domain package knows nothing about the CLI framework and swapping it later only touches that layer.
* `internal/clicfg` and `internal/config` do not import each other. `Settings` is passed down by the command, never read from a global.
* Domain packages take their dependencies as arguments (a `*slog.Logger`, a GitHub client interface, a `Settings` value). No package-level logger, client or config singleton.
* Interfaces are defined where they are consumed, so a domain package can be tested without the real GitHub client.
* No import cycles, and no cross-domain imports without a reason recorded here. If two domain packages need each other, the shared part is a third, smaller package.

## 4. Where code goes

| Code | Home |
|---|---|
| Root command, global flags, registering commands | `cmd/` |
| Flag parsing, `RunE`, help text for a command | `internal/cli/<cmd>/` |
| Anything with a branch worth testing beyond flag handling | a domain package in `internal/`, never in `cmd/` or a command package |
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
* **The `cmd/` boundary is lint-enforced.** A second depguard rule scoped to `cmd/` (`files: ["**/cmd/**"]`, strict) allows only `$gostd`, `github.com/spf13/cobra`, `github.com/rancher/orbs-automation/cmd`, `.../internal/cli` and `.../internal/errs` (depguard matches by prefix, so subpackages are covered). A matching rule for `main.go` allows only `$gostd` and `github.com/rancher/orbs-automation/cmd`. A third rule, `nocobra`, denies `github.com/spf13/cobra` in every file outside `cmd/`, `internal/cli/`, `main.go` and tests. The rest of dependency direction (section 3) is by review for now. A depguard rule per package can be added once there are enough packages to be worth it.

## Open questions

* Whether `internal/github` wraps `go-gh` and `go-github` behind one interface, or exposes two. Decided with the GitHub access spike (IMPLEMENTATION §3).
* Whether generated code (for example JSON schema from config types) gets its own directory.
