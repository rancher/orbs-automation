# Repos with more than one Go module

**Status:** a sketch. Nothing here is built beyond what [Where it shows today](#where-it-shows-today) says. It records how the pieces gero has could be extended to read and match `go.mod` files other than the one at the root of a repo, so the next case that needs it starts from a design and not from a guess.

## Why it matters

A repo can hold several modules, each with its own `go.mod`, its own `go` and `toolchain` lines and its own `require` lines. Rancher has `pkg/apis`, `pkg/client` and `pkg/plan` beside the root. BRO has `cmd/tool`, which requires wrangler and lasso indirectly. A producer can too: `ob-charts-tool` releases a module in a subdirectory (`helmtools/go.mod`, tagged `helmtools/v0.1.2`). Anything gero does with a `go.mod` has to say which one.

## Where it shows today

| Piece | What it does with several modules | Built |
|---|---|---|
| A **Holding**'s `at` list | One location for each module file that requires the module, each with its own `lines`. Rancher's wrangler Holding lists `go.mod`, `pkg/apis/go.mod`, `pkg/client/go.mod` and `pkg/plan/go.mod`. All are written together and must agree (`locations-differ` warns when they do not). | yes |
| A **virtual Holding** (a pin) | Takes the pinning Holding's files, and skips a file that does not require the pinned module, as a read finds no such module. | yes |
| **goBump** | Runs `go mod tidy` once in the directory of each `go.mod` the Holdings wrote, root first. | yes |
| **`{goToolchain}`** | Reads the **root** `go.mod` only. | root only |
| A pin's **`spec.pins.file`** | One file of the pinning release, `go.mod` by default. | one file |
| A **step**'s directory | Always the repo root. | root only |
| `replace` and `go.work` | Not modelled. | no |

So the **matching** of module files to what they hold is already the Holdings' `at` list, and what is missing is **reading facts other than a require line** from a module that is not the root: its toolchain, and the file a pin reads.

## The sketch

### 1. Which modules a repo has

Do not add a list. The modules gero acts on are the `go.mod` files that Holdings of format `gomod` name in `at`, and that is already the set it reads and writes. A module no Holding names is one gero has no business changing. A check can use it too: `gero repo status` could say that a module on the host that requires a held module is in no Holding's `at`, which is a place a bump will miss.

*Cost of the alternative:* a `modules:` list on the Project repeats what the Holdings say and gets out of date as they do.

### 2. The toolchain of a module other than the root

An Edit's `env` is worked out once for a target. Two ways to name another module:

* **A step has a directory** (recommended): `{run: [go, generate], dir: cmd/tool}`. `{goToolchain}` is then read from the `go.mod` **in the step's directory**, and falls back to the root's when that directory has none. A step with no `dir` is as today. The strategy runner has no directory of its own, so goBump passes the directory of each module it tidies and gets that module's toolchain.
* **A variable with a file**: `GOTOOLCHAIN: "{goToolchain|file:cmd/tool/go.mod}"`. Explicit, and it repeats the path in the env and again in the step.

The first fits how people think ("generate in `cmd/tool`") and needs one new field on a step. The cost is that `EnvOf` becomes per step and per directory, not once per target: it caches by directory, so each `go.mod` is still read once.

A repo whose modules pin **different** toolchains makes the root's wrong for a step in a nested module. The sketch above is the fix, and until it is built an Edit that needs it names the file by hand or runs one step per module.

### 3. The file a pin reads, in a producer with several modules

`spec.pins.file` defaults to `go.mod`, which is the root. For a module released from a subdirectory the file is under the directory its tag prefix names: a stream with `tagPattern: "helmtools/v*"` has its module in `helmtools/`, so the default should be `helmtools/go.mod`, derived from the prefix (`chain.TagPrefix`) rather than written. An Artifact that pins from a nested module in a stream with no prefix names the file as today.

*Check for it when built:* the file read must exist at the tag, and a pin that cannot find the pinned module in it says which file it read.

### 4. Matching versions across the modules of one consumer

Rancher aligns `pkg/client` and `pkg/apis` to the root after every bump (a `sync-deps` hook). Two reasons a nested module drifts: a Holding lists only some of the files, or a file requires the module only indirectly. The cheaper answers are in place: list every module file in the Holding, and read `locations-differ` as the warning. A **`goSync` strategy** would be a later step: after the Holdings are written, it makes each nested module that requires a module the root requires hold the root's version, and tidies. It belongs only if listing the files proves too much to maintain.

### 5. `replace` and `go.work`

A `replace` for the module a Holding writes makes the `require` line have no effect on a build. Today gero writes it anyway. A cheap check can read the `go.mod` it is about to write and **warn** that the module is replaced there (the `replace ../../` of BRO's `cmd/tool`, for one). `go.work` is out: nothing here uses it, and a workspace changes which `go.mod` wins.

### 6. Fetching modules from the host

The check reads files from the host at the branch head. Each module file costs one read of the host, and the status trace already keeps a file it has read for the rest of a branch (`memoTree`). Reading the toolchain of a nested module (section 2) adds at most one read for each directory a step names, and `EnvOf` would keep them the same way.

## Order, if it is built

1. The step's `dir`, with `{goToolchain}` read in it (section 2): the one piece the BRO case already points at.
2. The derived pin file for a prefixed stream (section 3).
3. The `replace` warning (section 5).
4. The status check for a module no Holding names (section 1).
5. `goSync` (section 4), only on demand.

## Questions left

* Should `dir` be on a step only, or also on an Edit, as a default for all its steps?
* Is the toolchain of a nested module the root's when the nested `go.mod` has no `go` line, or none?
* When two modules of one repo pin different toolchains and one Edit has steps in both, is one `env` enough, or does each step need its own?
