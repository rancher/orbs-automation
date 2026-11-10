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

## A release that pins another

An Artifact that `pins` another ([CONFIG_UNITS](CONFIG_UNITS.md#o12-how-do-two-packages-say-their-versions-go-together)) makes a **virtual** Holding of the pinned Artifact for each of its Holdings in a consumer, the **companions** of the Holding. A route that ships the pinning Artifact writes its Holdings and their companions in one run:

* **The release of a companion is not the latest of its stream.** It is the version the pinned module has in the file of the pinning release (`go.mod` at the tag, read from the host), written as the tag of the pinned Artifact's stream. It must be a release of that stream and exist, as a release of any gomod Artifact does; a version with no tag, a pseudo-version for one, is a blocker that says so
* **The check is a floor.** A companion holds what it must when the consumer's value is the required version or newer. Older is written. Newer is left as it is, and the target has a warning that the consumer holds a newer version than the pin resolves. It is never a downgrade and never blocked
* **The same targets.** A companion is for the lines of the Holding it comes from, in its files. A file that does not require the pinned module is skipped when it is read
* **Edits.** A companion follows the Edits of the Holding it comes from, so a `goBump` Edit tidies once after both `require` lines are written, as one unit
* **Shipping the pinned Artifact itself.** A virtual Holding of the route's own producer is a Holding of the route like any other: `gero ship lasso v0.2.7 --to bro` writes that tag in the files the pins make, as a floor, so it raises a lower version and keeps a higher one. Only the companions of another route's Holdings take their release from the pin
* **The engine's part.** A Holding's release is worked out for it, and not taken as the one tag of the run: `ReleaseOf` takes the companion's own tag, and the check says whether a value is older, the same or newer than it. The caller gives the engine the tags it derived, as it gives the runner, so the engine reads no host

## Edits

An edit's `env` is worked out **once for each target**, at the head of the target's branch, before any step or write ([CONFIG_SCHEMA](CONFIG_SCHEMA.md#edit-provisional-shape)): each value is rendered, a variable is read from the branch only when a value uses it, and an entry that renders empty is dropped. Every step of the edits of the target runs in it, and so does every strategy, through the runner it is given, so goBump's `go mod tidy` is run in it too. `{goToolchain}` reads `go.mod` of the tree (the host's at the head for the check, the clean worktree for a run), and for a `go` line with no patch asks a list of Go releases the caller gives the engine, as it gives the runner; a line it cannot resolve, a go.mod it cannot read and two edits that set one name differently are errors, which the check reports as blockers and a run reports before anything is written. A step the toolchain is too old for fails with Go's own message and nothing is committed.

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
