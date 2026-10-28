# Project Layout

**Status:** agreed. Nothing is built yet.

How Go code is organized in this repo. `gero` is a CLI, not a library, so the default is that nothing is importable from outside the module.

## 1. The `internal/` rule

* All Go packages live under `internal/`, except the `main` package under `cmd/gero/`. The compiler enforces that code outside this module cannot import `internal/`.
* There is no `pkg/` directory. Moving a package out of `internal/` later is cheap. Retracting a public API is not.
* **Promotion criteria.** A package leaves `internal/` only when all hold: a real external consumer exists, its API has survived at least one minor release unchanged, and it has no dependency on other `internal/` packages that are not themselves promoted. Promotion is a decision recorded in this spec, not a drive-by move.
* Data files (`config/`, `config-mock/`, `mock-repos/`) are not Go code and stay at the top level.

## 2. Layout

```
cmd/gero/            cobra wiring only: root, global flags, one file per command group
internal/clicfg/     CLI config loader -> Settings            (CLI_CONFIG)
internal/config/     application config: types, loader, validation, defaults
internal/ui/         output and prompt seam, plain-text implementation first   (CLI_OUTPUT)
internal/...         domain packages as they arrive: lines, bump, reconcile, github
```

Package names beyond these are decided with the code that needs them. A package is named for what it provides, not for a layer. No `util`, `common` or `helpers`.

## 3. Dependency direction

```
cmd/gero  ->  domain packages  ->  internal/config, internal/ui, ...
    \                                     ^
     `--> internal/clicfg  (Settings)     |
                                          |
          leaf packages import nothing from internal/
```

* `cmd/gero` may import any `internal/` package. Nothing imports `cmd/gero`.
* `internal/clicfg` and `internal/config` do not import each other. `Settings` is passed down by the command, never read from a global.
* Domain packages take their dependencies as arguments (a `*slog.Logger`, a GitHub client interface, a `Settings` value). No package-level logger, client or config singleton.
* Interfaces are defined where they are consumed, so a domain package can be tested without the real GitHub client.
* No import cycles, and no cross-domain imports without a reason recorded here. If two domain packages need each other, the shared part is a third, smaller package.

## 4. Where code goes

| Code | Home |
|---|---|
| Flag parsing, `RunE`, help text | `cmd/gero/` |
| Anything with a branch worth testing | `internal/`, never in `cmd/gero/` |
| Test helpers | `_test.go` files, or a `internal/<pkg>/<pkg>test` package if several packages share them |
| Golden files and fixtures | `testdata/` beside the package that uses them |

## 5. Enforcement

* The `internal/` rule is compiler-enforced.
* Imports of third-party modules are checked by the depguard allow-list ([DEPENDENCY_POLICY](DEPENDENCY_POLICY.md)).
* Dependency direction (section 3) is by review for now. A depguard rule per package can be added once there are enough packages to be worth it.

## Open questions

* Whether `internal/github` wraps `go-gh` and `go-github` behind one interface, or exposes two. Decided with the GitHub access spike (IMPLEMENTATION §3).
* Whether generated code (for example JSON schema from config types) gets its own directory.
