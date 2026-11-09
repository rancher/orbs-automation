# Specs

Small, focused documents for how `gero` is built. [VISION](../VISION.md) is why, [ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md) is the product shape, [PHASES](../PHASES.md) is what and when. A spec covers one narrow how-we-build-it topic, so a commit or test can cite it. Read the [docs rules](../README.md) before editing one. [design/](../design/CHAIN_MODEL.md) is newer than the specs: where the two disagree, `design/` is right and the spec is stale.

| Spec | Covers |
|---|---|
| [PROJECT_LAYOUT](PROJECT_LAYOUT.md) | Directory and package layout, the `internal/` rule, dependency direction, tasks, the lint-enforced boundaries |
| [CLI_FRAMEWORK](CLI_FRAMEWORK.md) | cobra, command tree, global flags, context gating, exit codes |
| [CLI_CONFIG](CLI_CONFIG.md) | CLI config versus application config, sources and precedence, user file, locating central config |
| [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md) | Workstation versus CI for commands that run in both: detection, the actor (PR author, commit identity), push and signing, the write-command contract |
| [LINES_RESOLVE](LINES_RESOLVE.md) | `gero lines resolve`: the release-line map, each Rancher line joined to the projects that feed it and follow it, and the gaps |
| [CHAIN_TRACING](CHAIN_TRACING.md) | Following a release chain through every hop, including the charts and Assets hops the release team runs: `managedBy` with `owned`, observed edges, tag schemes and the shared release scheme, stages (later); Assets and Rancher implemented |
| [REPO_STATUS](REPO_STATUS.md) | `gero repo status`: the sections of the report and what each answers, the state and next tags of a line, and who holds each release |
| [TASK_COMMANDS](TASK_COMMANDS.md) | The steps of `ship` as commands of their own: `repo branch` and `repo tag`, the use cases for the other steps, and the rules they share |
| [PUSH_TO_REPO](PUSH_TO_REPO.md) | The write half of a downstream bump: the work branch in the engineer's clone, remotes, branch and PR identity, per-target outcomes, what changes from today's scripts |
| [CI_SHIP](CI_SHIP.md) | `gero ci:ship`: the same bump as `gero ship` run by a workflow as the bot, what it shares and what CI adds |
| [METADATA](METADATA.md) | Labels and hidden markers (draft, not built): the marker as the record, label classes, owned versus shared repos, control labels |
| [WORKSPACE](WORKSPACE.md) | The one door from settings to the world: the central config source, current repo, resolving repo paths |
| [APP_CONFIG](APP_CONFIG.md) | Application config: document shape, kinds, placement, validation |
| [CLI_OUTPUT](CLI_OUTPUT.md) | stdout and stderr, `--json`, terminal capabilities and color, styling (`internal/ui/style`), logging, the interactive boundary |
| [GITHUB_ACCESS](GITHUB_ACCESS.md) | Token and host, REST, local git, subprocesses, consumer-side interfaces |
| [CENTRAL_BUNDLE](CENTRAL_BUNDLE.md) | Shipping the central config and templates as one release file: compile command, verified fetch, cache, source precedence |
| [IMAGES_EXIST](IMAGES_EXIST.md) | Port of `validate_image_exists`: `gero repo images exist`, the `images` config, the registry client |
| [SHELL_SETUP](SHELL_SETUP.md) | `gero shell install`: the marked profile block, completion, and the wrapper that lets `gero cd` change directory |
| [REPL](REPL.md) | `gero repl`: an interactive prompt over the same commands, its few shell-like commands (`pwd`, `ls`, `cd`, `clear`, `shell`, `exit`), the working directory as state, the line editor with history and completion, the prompt template (`repl.prompt`) |
| [DOCTOR](DOCTOR.md) | `gero doctor`: workstation health checks, statuses, `--online`, output, exit codes |
| [SELF_UPDATE](SELF_UPDATE.md) | `gero doctor self-update`: replacing the binary with the newest release, only after its manifest and archive verify against a pinned Sigstore signer; what the release workflow must publish |
| [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md) | Stdlib-first, approved modules, the depguard allow-list |
| [TESTING](TESTING.md) | How the CLI layer is tested |
| [CHECK_SEMVER](CHECK_SEMVER.md) | Port of the shared `check-semver` script: decisions, command, library, test plan |
| [BRANCH_TAGS](BRANCH_TAGS.md) | Port of the shared `branch-tags.sh` script: behavior, command, library, test plan |

## Conventions

* Each spec starts with a one-line **Status**: `draft` (proposal), `agreed` (decided, not built), or `implemented` (code exists and matches). A partly built spec says which parts, in one sentence, and lists the rest under **Not built yet** at the end.
* A spec records what is decided and why, in the present tense. It does not repeat [PHASES](../PHASES.md), narrate history or log progress.
* **Write a spec once the shape settles**, not while it is moving ([docs rules](../README.md)). Most of these were written after the code.
* Once a spec is `implemented`, the code and tests are the source of truth. Trim the spec to the rationale the code cannot carry, or delete it.
* A spec for a ported script describes a **faithful port** as the default, and lists approved improvements in a closing section, "Breaking changes that will improve the script after port". Improvements are opt-in until a release deliberately makes one the default.
* Each spec ends with an **Open questions** section, which is the only place open items live.
