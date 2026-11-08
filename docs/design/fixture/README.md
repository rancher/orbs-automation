# Fixture: the proposed units, written out

**Status:** working fixture, 2026-11-07. The seven projects in the shape [CONFIG_UNITS](../CONFIG_UNITS.md) proposes, plus two invented non-Rancher examples in [`examples/`](examples/). It tests the shape; nothing loads it. All `refs` resolve to a document of the right kind (checked by a throwaway program, 0 dangling). A Holding is one document with an `at` block and a `holds` block.

| File | Documents |
|---|---|
| `kuberlr-kubectl.yaml` | Project, Stream, Artifact |
| `shell.yaml` | Project, Stream, Artifact |
| `scc-operator.yaml` | Project, Stream, Artifact |
| `backup-restore-operator.yaml` | Project, Stream, 3 Artifacts, Holding |
| `rancher-assets.yaml` | Project, Stream, Artifact, Holding |
| `rancher-charts.yaml` | Project, one chart Stream, 2 Holdings, Edit |
| `rancher.yaml` | Project, Stream, Artifact, 4 Holdings, Edit |
| `rancher.lines.yaml` | RancherLines, as in the real config |
| `examples/library.yaml` | a Go library, two services holding it in `go.mod` |
| `examples/monorepo.yaml` | per-package tags, an image pinned in a Dockerfile |
| `examples/external.yaml` | a third-party SDK as an external Project, tracked by a Holding in BRO |

## The questions the tools must answer

| # | Question | Units it reads |
|---|---|---|
| 1 | What does this branch of a repo hold of each producer, and where? | Project (branch for a line), Holding |
| 2 | Is each held version stable? | Holding (`holds`, `form`), Artifact, Stream (scheme, stages) |
| 3 | Does each held version belong to its own line? | Stream (series), Holding |
| 4 | Which release of a producer should a branch hold? | Stream (scheme, lines), Holding (`accept`, `lines`) |
| 5 | Who consumes this producer? | Holdings that name its artifacts (derived) |
| 6 | Is a release complete? | Artifact (images in registries, files on the GitHub release) |
| 7 | What would shipping do? | Holding, Edit, Project (PR conventions) |
| 8 | Which line is a branch, and which branch is a line? | Project (branches), RancherLines |
| 9 | Is the config consistent? | every `refs`, names, series overlap |

## Walking the commands through the units

| Command | Needs |
|---|---|
| `repo info` | Project, Stream, Artifacts |
| `repo tag` | Stream (scheme, `requiresPrerelease`), Project (branches) |
| `repo status` | Project, Stream, and Holdings for what each consumer holds |
| `lines resolve` | RancherLines, Project (branches), Stream (series), Holdings |
| `repo images exist` | Artifacts of `kind: image`, and the registry rule by release kind |
| `ship` | Holding, Edit, Project (PR conventions, push target) |
| `config validate` | every `refs`, names, series overlap |

Every command finds what it needs. None needs a field that only exists on a Project today.

## What it showed

See [CONFIG_UNITS](../CONFIG_UNITS.md#what-the-fixture-showed).
