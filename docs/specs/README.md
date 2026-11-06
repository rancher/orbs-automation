# Specs

Small, focused documents for how `gero` is built. [VISION](../VISION.md) is why, [ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md) is the product shape, [PHASES](../PHASES.md) is what and when. A spec covers one narrow how-we-build-it topic, so a commit or test can cite it. Read the [docs rules](../README.md) before editing one.

| Spec | Covers |
|---|---|
| [PROJECT_LAYOUT](PROJECT_LAYOUT.md) | Directory and package layout, the `internal/` rule, dependency direction |
| [CLI_FRAMEWORK](CLI_FRAMEWORK.md) | cobra, command tree, global flags, context gating, exit codes |
| [CLI_CONFIG](CLI_CONFIG.md) | CLI config versus application config, sources and precedence, user file, locating central config |
| [EXECUTION_CONTEXT](EXECUTION_CONTEXT.md) | Workstation versus CI for commands that run in both: detection, the actor (PR author, commit identity), push and signing, the write-command contract |
| [PUSH_TO_REPO](PUSH_TO_REPO.md) | The write half of a downstream bump: throwaway clone, remotes, branch and PR identity, per-target outcomes, what changes from today's scripts |
| [METADATA](METADATA.md) | Labels and hidden markers: the marker as the record, label classes, owned versus shared repos, control labels |
| [WORKSPACE](WORKSPACE.md) | The one door from settings to the world: inferred defaults, current repo, resolving repo paths |
| [APP_CONFIG](APP_CONFIG.md) | Application config: document shape, kinds, placement, validation |
| [CLI_OUTPUT](CLI_OUTPUT.md) | stdout and stderr, `--json`, color, logging, the `internal/ui` seam |
| [GITHUB_ACCESS](GITHUB_ACCESS.md) | Token and host, REST, local git, subprocesses, consumer-side interfaces |
| [CENTRAL_BUNDLE](CENTRAL_BUNDLE.md) | Shipping the central config and templates as one release file: compile command, verified fetch, cache, source precedence |
| [IMAGES_EXIST](IMAGES_EXIST.md) | Port of `validate_image_exists`: `gero repo images exist`, the `images` config, the registry client |
| [SHELL_SETUP](SHELL_SETUP.md) | `gero shell install`: the marked profile block, completion, and the wrapper that lets `gero cd` change directory |
| [DOCTOR](DOCTOR.md) | `gero doctor`: workstation health checks, statuses, `--online`, output, exit codes |
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
