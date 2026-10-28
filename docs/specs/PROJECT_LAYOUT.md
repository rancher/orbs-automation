# Project Layout

**Status:** agreed. Nothing is built yet.

How Go code is organized in this repo. `gero` is a CLI, not a library, so the default is that nothing is importable from outside the module.

## 1. The `internal/` rule

* All Go packages live under `internal/`, except the `main` package (`main.go` at the repo root) and the cobra wiring under `cmd/`. The compiler enforces that code outside this module cannot import `internal/`.
* There is no `pkg/` directory. Moving a package out of `internal/` later is cheap. Retracting a public API is not.
* **Promotion criteria.** A package leaves `internal/` only when all hold: a real external consumer exists, its API has survived at least one minor release unchanged, and it has no dependency on other `internal/` packages that are not themselves promoted. Promotion is a decision recorded in this spec, not a drive-by move.
* Data files (`config/`, `config-mock/`, `mock-repos/`) are not Go code and stay at the top level.

## 2. Layout

```
main.go              entry point: calls cmd.Execute(), nothing else
cmd/                 cobra wiring only: root command, global flags
cmd/<group>/         one subdirectory per command group (for example `cmd/ci/` for the CI-only commands)
internal/clicfg/     CLI config loader -> Settings            (CLI_CONFIG)
internal/config/     application config: types, loader, validation, defaults
internal/ui/         output and prompt seam, plain-text implementation first   (CLI_OUTPUT)
internal/...         domain packages as they arrive: lines, bump, reconcile, github
test/integration/    separate Go module (own go.mod) for integration tests     (TESTING)
```

**Decision: `cmd/` holds the root command, subdirectories are command groups.** The root command and global flags live in `cmd/` itself (package `cmd`), with `main.go` at the repo root, as `cobra-cli init` generates. A subdirectory under `cmd/` is not a binary (there is one binary, `gero`); it is a command group. Groups come in two kinds: a noun group is a real parent command (`gero config validate`), and a cobra help group (`cobra.Group`) lists top-level commands under one heading. `cmd/ci/` is the second kind, following `ob-charts-tool`: group definitions live in `cmd/groups/`, each group package keeps a `subCommandList`, prefixes its commands' `Use` with the group ID (`gero ci:reconcile`) and exports a register function. The CI-only commands share the "CI-Only Commands:" heading, so they stand out from the human-run noun-verb commands. A group stays a single file in `cmd/` until it needs its own directory. This replaces the earlier plan of `main` under `cmd/gero/`.

Package names beyond these are decided with the code that needs them. A package is named for what it provides, not for a layer. No `util`, `common` or `helpers`.

## 3. Dependency direction

```
cmd/...   ->  domain packages  ->  internal/config, internal/ui, ...
    \                                     ^
     `--> internal/clicfg  (Settings)     |
                                          |
          leaf packages import nothing from internal/
```

* `cmd/` and its subdirectories may import any `internal/` package. Nothing imports `cmd/`, except `main.go` importing the root package.
* `internal/clicfg` and `internal/config` do not import each other. `Settings` is passed down by the command, never read from a global.
* Domain packages take their dependencies as arguments (a `*slog.Logger`, a GitHub client interface, a `Settings` value). No package-level logger, client or config singleton.
* Interfaces are defined where they are consumed, so a domain package can be tested without the real GitHub client.
* No import cycles, and no cross-domain imports without a reason recorded here. If two domain packages need each other, the shared part is a third, smaller package.

## 4. Where code goes

| Code | Home |
|---|---|
| Flag parsing, `RunE`, help text | `cmd/`, `cmd/<group>/` |
| Anything with a branch worth testing | `internal/`, never in `cmd/` |
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
* Dependency direction (section 3) is by review for now. A depguard rule per package can be added once there are enough packages to be worth it.

## Open questions

* Whether `internal/github` wraps `go-gh` and `go-github` behind one interface, or exposes two. Decided with the GitHub access spike (IMPLEMENTATION §3).
* Whether generated code (for example JSON schema from config types) gets its own directory.
