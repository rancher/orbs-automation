# Chain Model

**Status:** implemented. The resolved model that commands and `ship` read, in `internal/chain`, built from the documents of [CONFIG_SCHEMA](CONFIG_SCHEMA.md). Where it sits is in [LAYERS](LAYERS.md).

## Why a package of its own

The kinds are small documents joined by `refs`: a Project does not know its Streams, a Stream does not know its Artifacts, a Holding is found by a project and an artifact. A command asks across them: which branch is line v2.15 of this project on, which line does this tag belong to, which images must exist, who holds a release of this project. No one document can answer, and the types in `internal/apis` stay data with their own rules, so the answers are given by a graph built once.

* Built from an `appconfig.Config` that already passed every rule, so it does not validate and never fails to build
* Read-only: nothing in it changes after `Build`
* Domain layer: it imports `apis`, `appconfig` and `platform`, and nothing of `cli` or `ui`
* Each type wraps its document (`Doc`) and adds links and answers; a command never needs to walk `refs` by name

## Shape

| Type | Holds | Answers |
|---|---|---|
| `Graph` | every Project, the lines, every Holding | a Project by name, by `owner/name` slug, or by a clone's remotes; every holding; the lines in order |
| `Project` | the Project document, its Streams, the Holdings it makes and the Holdings made of it | `Steward`, `DefaultBranch`, `Branch(line)`, `BranchOfLine(line, stage)`, `LineOfBranch(branch)`, `ReleaseBranches`, `Held`, `Sourced` |
| `Stream` | the Stream document, its Project and Artifacts | `LineOf(tag)`, `Series(line)`, `Stages`, `Serves(line)` |
| `Artifact` | the Artifact document and its Stream | `Registries(stable)`, `AppliesTo(line)` |
| `Holding` | the Holding document, its holder and producer Projects, its Artifact or `from` | `AppliesTo(line)`, `Edits` |
| `Edit` | the Edit document and the Holdings it follows | `Steps(line)`, `Strategy` |

## Answers, and where each came from

The rules the graph applies, each in `internal/chain`.

| Answer | Rule |
|---|---|
| `Project.ManagedBy` | `managedBy` as written; none when unset, and none for an external repo |
| `Project.Owned(team)` | `managedBy` is the team given, which is the active team of the CLI settings ([CLI_CONFIG](../specs/CLI_CONFIG.md)); never for `shared`, for an unset `managedBy` or for an external repo |
| `Project.Shared` | `managedBy` is `shared`: no team owns it and every team may ship into it |
| `Project.Branch(line)` | the line's branch as written; omitted gives the default branch (and says so), `""` gives none yet, a stage map gives a branch for each stage |
| `Project.BranchOfLine(line, stage)` | the same, for a line id written with or without `v`, any case; a `single` layout gives the default branch for every line; a stage picks from the stage map |
| `Project.LineOfBranch(branch)` | the first line whose branch (or a stage's branch) is the branch |
| `Project.ReleaseBranches` | the default branch for `single`, else the distinct branches of the lines, in order |
| `Stream.Series(line)` | the series written for the line; there is none for `calverLineStage`, whose tag carries the line |
| `Stream.LineOf(tag)` | the line whose series holds the tag, by the scheme's grammar; a `universal` stream has no line of its own; a calver tag names it |
| `Stream.Stages` | the stages written, else the scheme's: `calverLineStage` gives `dev` (a prerelease) and `stable`, `chartVersion` takes the branch layout's stages, `semver` gives none |
| `Stream.Serves(line)` | `universal`, or the line is one of the stream's lines |
| `Artifact.Registries(stable)` | the registries written, else Docker Hub and Prime staging, and Prime production for a stable ([CONFIG_UNITS](CONFIG_UNITS.md), registries are code) |
| `Artifact.AppliesTo(line)` | the artifact's `lines` range holds the line; a line that does not read as one counts as in range |
| `Holding.AppliesTo(line)` | the holding's `holds.lines` range holds the line, and the line is one the holder and the producer both list |
| `Edit.Steps(line)` | the pre and post steps whose `since` and `until` hold the line, as `chain.Step` (the graph's own type, so `bump` and `steprun` do not import the API version) |
| `Line.PhaseAt` | in `apis/lines`, shared by every version |

No answer is stored in the config that can be worked out from it. There is no implicit image: an Artifact is written for each thing a release produces.

## What is not here

* Writing a holding (render its form, put it at its place) is the update engine's, which reads this graph
* A named strategy of an Edit (`chartsPackageBump`) is code the engine runs; the graph only names it
* Validation: the engine of [CONFIG_ENGINE](CONFIG_ENGINE.md) did it before the graph was built

## One adapter

`chain` is the only domain package that reads a version of the API. The names the domain uses (layouts, what a holding holds, `accept`, registry roles) are `chain` constants; the scheme and stage names belong to `tagscheme` and `calver`; and `names_test.go` checks each is the name the config accepts. A new version changes `appconfig`, `chain` and `chainload`. The `oneversion` rule in `.golangci.yml` denies `apis/v1alpha2` anywhere else, except in tests. What every version shares, such as a line and its dates, is in `apis/lines`.
