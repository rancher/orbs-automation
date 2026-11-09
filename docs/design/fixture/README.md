# Fixture: the proposed units, written out

**Status:** working fixture, 2026-11-08. The projects in the shape [CONFIG_UNITS](../CONFIG_UNITS.md) proposes: the seven of the baseline, the frameworks team's wrangler and lasso, and examples in [`examples/`](examples/). The Go tests load it (`internal/apis/v1alpha2`, `internal/appconfig`, `internal/chain`), so it is the contract the loader meets, and it loads whole. What the fixture describes and nothing reads yet is in the status of [CONFIG_UNITS](../CONFIG_UNITS.md). A Holding is one document with an `at` block (a location or a list) and a `holds` block.

| File | Documents |
|---|---|
| `kuberlr-kubectl.yaml` | Project, Stream, Artifact |
| `shell.yaml` | Project, Stream, Artifact |
| `scc-operator.yaml` | Project, Stream, Artifact |
| `backup-restore-operator.yaml` | Project, Stream, 3 Artifacts, 4 Holdings (kuberlr, wrangler 3.7 and 3.3, lasso) |
| `rancher-assets.yaml` | Project, Stream, Artifact, Holding |
| `rancher-charts.yaml` | Project, one chart Stream, 2 Holdings, Edit |
| `rancher.yaml` | Project, Stream, Artifact, 9 Holdings (build.yaml, charts branch, wrangler for each minor, lasso), Edit |
| `wrangler.yaml` | Project (unowned), a Stream and `gomod` Artifact for each tracked minor (v3.8, v3.7, v3.6, v3.3) |
| `lasso.yaml` | Project (unowned), one Stream and `gomod` Artifact for v0.2 |
| `rancher.lines.yaml` | RancherLines, as in the real config |
| `examples/library.yaml` | a library, two services holding it in their Dockerfiles |
| `examples/monorepo.yaml` | per-package tags, an image pinned in a Dockerfile |
| `examples/gomod.yaml` | a prefix-tagged Go module held by Assets' `go.mod` with a `goBump` Edit |
| `examples/external.yaml` | a third-party SDK as an external Project, tracked by a Holding in BRO's chart values |

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
