# Update Engine

**Status:** implemented, except the strategy `chartsPackageBump`, which is declared and not built. The engine that ships a release into the holdings of a consumer, in `internal/bump`, built on the [chain model](CHAIN_MODEL.md). `gero ship` and `gero repo status` read it.

## What it does

A release of a producer is a tag of one of its streams. A consumer holds something of it in a file, through a **Holding** of kind `release`. The engine:

* **reads** what a consumer branch holds, and says how that compares with a release (the read side, which never writes)
* **writes** the release into the consumer's files (the write side)
* **runs** the Edits that follow the holdings, around the write

## Routes

A **route** is a producer and a consumer it has release holdings in: every Holding of kind `release` whose holder is the consumer and whose artifact is made by a stream of the producer. A producer has one route for each consumer. One run of `gero ship` ships down one route, so every holding of it is written together, in one pull request for each target branch, and a consumer that holds two files of one release (BRO's two chart URLs) is never left half updated.

* `RoutesFrom(producer)` lists the routes out of a project, `RoutesInto(consumer)` the ones into it, each in the order the holdings are read
* A holding of kind `commit` or `branch` is not a route: gero observes what it holds and never writes it

## Targets

A route has one target for each branch of the consumer a release goes to. For a line, a holding's branch is the consumer's branch of the line for the holding's `stage` (none for a holder with one branch of each line). A holding is for a line only if it applies to it ([CHAIN_MODEL](CHAIN_MODEL.md)); the holdings of a target are the ones that apply to its line.

* A producer whose stream is universal sends every release to every release branch of the consumer, narrowed to the branches of a holding's stage when it has one
* A producer with lines sends a release to the branch of the line the tag is in; a tag in no line has no target
* Holdings of a route that name different stages have different branches, so they are different targets

## A holding written

For a target branch, a holding gives:

| Part | Is |
|---|---|
| file | `at.file`, rendered with `line`, `stage` and `branch` |
| key | `at.key`, rendered the same, then read as the format's own path |
| value | `holds.form`, rendered with the variables of the release |

The variables of a form are `image` (the artifact's repository path), `tag`, `version` (the tag without its v), `url` and `file` (for a file artifact, the rendered file name and its address among the release's files), `sourceRepo` (the producer's `owner/name`), and `commit`, `base` and `upstream` for the kinds that read them. A form that uses a modifier is rendered with it; it is written only with ones that can be undone, so the value held can be read back as a tag.

* **Formats built:** `yaml`, read and replaced in place without disturbing the rest of the file, by the path of the [schema](CONFIG_SCHEMA.md#templates-formats-and-paths): keys, quoted keys for a dot inside one, `[n]` for a list item, a leading dot optional. A holding of another format is reported as not built, and is an error when it is a release holding the engine would write
* **Check:** reads the value, renders the form for the release, and says what is held, what is wanted, whether they are the same, and how the versions in them compare (newer, same, older, or unknown when either has no version)
* **Apply:** replaces the value when it differs, and says what changed, one change for each holding that changed

## Edits

The edits that follow the holdings of a route run around the write, for the line of the target branch, each edit once:

1. every edit's `preEdit` steps whose range holds the line, in the order the edits are read
2. the write of every holding
3. every edit's `strategy`, a named rule that needs code
4. every edit's `postEdit` steps whose range holds the line

A strategy is code the engine has, found by its name: building the engine configures each strategy with its edit's `with` values and reports a mistake there, as the config loads. `chartsPackageBump` is named in the real config and is not built: it is registered so that the config is valid, and a run that needs it stops before anything is written, saying it is declared and not built.

The steps are run by a runner the caller supplies, so the engine starts no process. After each unit (a step, the write, a strategy) a checkpoint is called, so the caller can commit what it left behind.

## Not here

* Choosing the release and the targets, the pull request, the commits and the push are `internal/ship`'s
* Reading a commit or a branch holding, and the `dockerfile` format, are not built
