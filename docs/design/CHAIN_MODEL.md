# Chain Model

**Status:** implemented. The resolved model that commands and `ship` read, in `internal/chain`, built from the documents of [CONFIG_SCHEMA](CONFIG_SCHEMA.md). Where it sits is in [LAYERS](LAYERS.md).

## Why a package of its own

The kinds are small documents joined by `refs`: a Project does not know its Streams, a Stream does not know its Artifacts, a Holding is found by a project and an artifact. A command asks across them: which branch is line v2.15 of this project on, which line does this tag belong to, which images must exist, who holds a release of this project. No one document can answer, and the types in `internal/apis` stay data with their own rules, so the answers are given by a graph built once.

* Built from an `appconfig.Config` that already passed every rule, so it does not validate and never fails to build
* Read-only: nothing in it changes after `Build`
* Domain layer: it imports `apis`, `appconfig` and `platform`, and nothing of `cli` or `ui`
* Each type wraps its document (`Doc`) and adds links and answers; a command never needs to walk `refs` by name
* Only this package reads a `Doc`: lint refuses it anywhere else, tests included, so a command asks the type, and a missing answer is added here

## Shape

| Type | Holds | Answers |
|---|---|---|
| `Graph` | every Project, the lines, every Holding | a Project by name, by `owner/name` slug, or by a clone's remotes; every holding; the lines in order; a line by its id, `Line(id)` |
| `Project` | the Project document, its Streams, the Holdings it makes and the Holdings made of it | `ManagedBy`, `Owned(team)`, `Shared`, `Named(key)`, `DefaultBranch`, `Branch(line)`, `BranchOfLine(line, stage)`, `LineOfBranch(branch)`, `ResolveLineOrBranch(text)`, `ReleaseBranches`, `ReleaseTargets`, `ReleaseTargetsOf(line)`, `ListsLine(line)`, `LineID(line)`, `StreamListsLine(line)`, `LinesInTags`, `SeriesOf(line)`, `OnlyStream`, `StreamOf(tag)`, `ReleaseVersion(tag)`, `DestinationHoldings(line)`, `Held`, `Sourced` |
| `Stream` | the Stream document, its Project and Artifacts | `LineOf(tag)`, `Series(line)`, `Stages`, `Serves(line)`, `TagMatches(tag)`, `HasRelease(value)`, `BareTag(tag)`, `Version(tag)` |
| `Artifact` | the Artifact document and its Stream | `Registries(stable)`, `AppliesTo(line)`, `Ranged` |
| `Holding` | the Holding document, its holder and producer Projects, its Artifact or `from` | `AppliesTo(line)`, `HoldsLine(line)`, `Stage`, `Form`, `AcceptsAll`, `Locations`, `Edits` |
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
| `Project.ResolveLineOrBranch(text)` | what a person typed to say where a release is made: a line, with or without the `v`, or a release branch of the project. It gives the branch and the line the branch is of. A line must be one the project lists or one a stream gives a series; text that is neither, or a line with no branch yet, is an error naming the lines the project has |
| `Project.ReleaseTargets` | the release branches of the project, as `chain.ReleaseTarget{Line, Series, Stage, Branch, IsDefault}`: a row for each listed line, and for each stage of a line that has a branch for each. A `single` layout has the one branch, with a row for each line its streams list, or one row of no line when they list none |
| `Project.ReleaseTargetsOf(line)` | the rows of `ReleaseTargets` for one line, written with or without the `v`, in any case |
| `Project.SeriesOf(line)` | the series the first stream that gives the line one writes for it |
| `Graph.Line(id)` | the line of the lines document the id names, with or without the `v`, in any case |
| `Project.LineID(line)` | the id the project lists the line under, for a line typed with or without the `v`, in any case; none when it does not list it. Like `ListsLine` it knows only the lines listed for branches |
| `Project.LinesInTags` | a `single` layout with a stream that lists lines: the project finds its lines in its tags, and `ReleaseTargets` has a row for each |
| `Project.ListsLine(line)` | the project lists the line itself for its branches. A `single` layout lists none, though `BranchOfLine` answers for every line |
| `Project.StreamListsLine(line)` | a stream of the project lists the line itself, which a `universal` stream does not |
| `Project.OnlyStream` | the project's one stream; an error wrapping `ErrNoStream` or `ErrSeveralStreams` when it has none or more than one |
| `Project.StreamOf(tag)`, `Project.ReleaseVersion(tag)` | the first stream whose pattern the tag matches and none of whose excludes it matches; the version of the release, which is the tag without that stream's prefix and without a leading `v`, or the tag without its `v` when no stream matches |
| `Project.Named(key)` | the key is the project's config name, its `owner/name` slug or one of its aliases, whatever the case. The rules for names make these unique, so it agrees with how `Graph.Project` finds a project |
| `Project.DestinationHoldings(line)` | the holdings of a release of the project in the final destination that are for the line |
| `Stream.HasRelease(value)` | the value is a tag of the stream, or a tag without the prefix a Go module goes without |
| `Stream.BareTag(tag)`, `Stream.Version(tag)` | the tag without the prefix its pattern puts before the version, and that without a leading `v`. `chain.BareTag(pattern, tag)` and `chain.Version(pattern, tag)` are the same for a pattern alone |
| `Stream.Series(line)` | the series written for the line; there is none for `calverLineStage`, whose tag carries the line |
| `Stream.LineOf(tag)` | the line whose series holds the tag, by the scheme's grammar; a `universal` stream has no line of its own; a calver tag names it |
| `Stream.Stages` | the stages written, else the scheme's: `calverLineStage` gives `dev` (a prerelease) and `stable`, `chartVersion` takes the branch layout's stages, `semver` gives none |
| `Stream.Serves(line)` | `universal`, or the line is one of the stream's lines |
| `Artifact.Registries(stable)` | the registries written, else Docker Hub and Prime staging, and Prime production for a stable ([CONFIG_UNITS](CONFIG_UNITS.md), registries are code) |
| `Artifact.AppliesTo(line)` | the artifact's `lines` range holds the line; a line that does not read as one counts as in range |
| `Artifact.Pins` and `Artifact.PinnedBy` | the Artifacts this one pins, as written in `refs.pins`, and the ones that pin it; an edge of the graph between Artifacts, and none when no Artifact pins ([CONFIG_UNITS](CONFIG_UNITS.md#o12-how-do-two-packages-say-their-versions-go-together)) |
| `Holding.Virtual`, `Holding.PinnedBy` | a virtual Holding is the one a pin makes in a consumer: the pinned Artifact, the files and lines of the Holding it comes from (`PinnedBy`), and no document of its own |
| `Holding.Pinned` | the virtual Holdings of the pinned Artifacts of a Holding, which a run that ships its Artifact writes beside it, and which follow its Edits |
| `Holding.HoldsLine(line)` | the holding's own `holds.lines` range holds the line, which is narrower than `AppliesTo` |
| `Holding.Stage`, `Holding.Form`, `Holding.AcceptsAll` | the stage and the form as written, and whether `holds.accept` is `all`, so that a prerelease is taken too |
| `Holding.Locations` | the places the holding is looked for, in the order written, as `chain.Location{File, Key, Format}` with `AppliesTo(line)` for the lines the file exists for. File and Key are templates, rendered by the update engine |
| `Artifact.Ranged` | the artifact's `lines` range is set, so it is for some lines only |
| `Holding.AppliesTo(line)` | the holding's `holds.lines` range holds the line, and the line is one the holder and the producer both list |
| `Edit.Steps(line)` | the pre and post steps whose `since` and `until` hold the line, as `chain.Step` (the graph's own type, so `bump` and `steprun` do not import the API version) |
| `Line.PhaseAt` | in `apis/lines`, shared by every version |

A virtual Holding is built with the graph, from each Holding of a pinning Artifact and each Artifact it pins, so a route, a target, a check and the validation of overlaps see it as a Holding like any other: it is a route out of the pinned Artifact's project, and `RoutesFrom` lists it. What is not worked out from the config is the release it holds: that is the version in the pinning release's file, read when a run ships ([UPDATE_ENGINE](UPDATE_ENGINE.md#a-release-that-pins-another)).

No answer is stored in the config that can be worked out from it. There is no implicit image: an Artifact is written for each thing a release produces.

## What is not here

* Writing a holding (render its form, put it at its place) is the update engine's, which reads this graph
* A named strategy of an Edit (`chartsPackageBump`) is code the engine runs; the graph only names it
* Validation: the engine of [CONFIG_ENGINE](CONFIG_ENGINE.md) did it before the graph was built

## One adapter

`chain` is the only domain package that reads a version of the API. The names the domain uses (layouts, what a holding holds, `accept`, registry roles) are `chain` constants; the scheme and stage names belong to `tagscheme` and `calver`; and `names_test.go` checks each is the name the config accepts. A new version changes `appconfig`, `chain` and `chainload`. The `oneversion` rule in `.golangci.yml` denies `apis/v1alpha2` anywhere else, except in tests. What every version shares, such as a line and its dates, is in `apis/lines`.
