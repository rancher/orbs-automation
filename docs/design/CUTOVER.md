# Cutover

**Status:** done, 2026-11-08. How every command moved from the `gero/v1alpha1` model (`internal/config` and its loader) to `gero/v1alpha2`, and what is deleted at the end. The graph it moves to is in [CHAIN_MODEL](CHAIN_MODEL.md).

## Decisions

* Commands and `ship` read a resolved graph (`internal/chain`), not the documents and not the old types
* The update engine is rebuilt on Holding and Edit: methods, edges, `allowedParams` and parameter binding go away. A Holding is the rule that writes and reads, writing its form at its place is the engine's default, and an Edit carries steps and named strategies
* The real `config/` is rewritten in v1alpha2 from the [fixture](fixture/README.md), and `config-mock/` is retired
* Command output may change where the model changes, and the command specs change with it
* `workspace` stays the one door. It keeps the directory, repo, clone, upstream and push parts, and the lookup of a Project by name, slug or the working directory's clone. `Config()` becomes `Chain()`, and `Updates()` builds the rebuilt engine

## The real config in the new shape

Where the real config and the fixture differ, the real config wins as to facts, and the settled choices are:

| Subject | Decision |
|---|---|
| Rancher images | `rancher/rancher` and `rancher/rancher-agent` for every line, `rancher/rancher-server` since v2.16 |
| Which releases Rancher takes | `accept: all` on its holdings of shell, scc-operator and assets, as in the fixture; the old default was `stable` |
| Charts' stable branches | the fixture's `release-v2.X`; the real config only has `dev-v2.X` and the names are to be checked |
| BRO's chart URL | the fixture's form: two file Artifacts and two Holdings with `{url}`; `{repo}` and `{tag (without V)}` are not variables |
| `kuberlr-kubectl` | in, as in the fixture; `ob-team-charts` is left out until its package is known |
| Series | written for every semver stream, as in the fixture |
| A line with no `branch` after its development | a warning, where the old loader gave an error |
| A Holding of an artifact of its own project | an error: no use is known, and a rule that is too strict is loosened without breaking a config, where one that is too loose breaks configs when it is tightened |

## Order

1. This design and the rules the old loader enforced that v1alpha2 lacks
2. The graph, a getter family at a time, tested on small documents and on the fixture
3. The real config in `config/v1alpha2/`, beside the old files: the old loader reads only the files at the root of `config/`, so it ignores the folder
4. `workspace.Chain()` beside `Config()`
5. Each consumer, leaf first: constants and dates, registry, the commands that list projects, the repo commands, `lines resolve`, then the engine and `ship`. Each package moves in its own commit with its tests moved to small documents
6. `Config()`, the old types, the old loader, `config-mock/` and the v1alpha1 files are deleted, `config/v1alpha2/` moves up to `config/`, and the lint doors name only `appconfig`
7. The specs and READMEs are rewritten to the new model (the docs refresh of 2026-11-08)

The old model stays in place and green until the last consumer has moved.

## Rules of the old loader not carried over

Closed in step 1: one `RancherLines` document; no spaces in a repo name or an image name; repo slugs unique whatever their case; image names unique within a project; a calver stream only on a single-layout project, and a stream with `lines` or `universal`; stages of a holding that its producer and holder have; `from` of a step; a project that holds its own artifact.

Not carried over, on purpose:

| Old rule | Why |
|---|---|
| a file's name must say its kind | a document is what it says it is, not where it is |
| update methods, `allowedParams`, `with` against `allowedParams`, edge params | the concepts are gone |
| a strategy is known and its `with` fits it | the engine checks this when it builds a plan, as `Edit` says |
| an edge's method exists on the consumer | the Holding is the method |
| `External` relationships and `notify` | there is no `External` kind; `notify` is not modelled |
| PR text variables from the project's own strategies | one set of variables for every project |
| a line without a branch after development is an error | a warning, as [CONSTRAINTS](CONSTRAINTS.md) says of lifecycle dates |
