# Specs

Small, focused documents for how `gero` is built. [VISION](../VISION.md) is why, [ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md) is the product shape, [PHASES](../PHASES.md) is what and when. A spec covers one narrow how-we-build-it topic, so a commit or test can cite it.

| Spec | Covers |
|---|---|
| [PROJECT_LAYOUT](PROJECT_LAYOUT.md) | Directory and package layout, the `internal/` rule, dependency direction |
| [CLI_FRAMEWORK](CLI_FRAMEWORK.md) | cobra, command tree, global flags, context gating, exit codes |
| [CLI_CONFIG](CLI_CONFIG.md) | CLI config versus application config, sources and precedence, user file, locating central config |
| [CLI_OUTPUT](CLI_OUTPUT.md) | stdout and stderr, `--json`, color, logging, the `internal/ui` seam |
| [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md) | Stdlib-first, approved modules, the depguard allow-list |
| [TESTING](TESTING.md) | How the CLI layer is tested |
| [CHECK_SEMVER](CHECK_SEMVER.md) | Port of the shared `check-semver` script: decisions, command, library, test plan |
| [BRANCH_TAGS](BRANCH_TAGS.md) | Port of the shared `branch-tags.sh` script: behavior, command, library, test plan |

## Conventions

* Each spec starts with a one-line **Status**: `draft` (proposal), `agreed` (decided, not built), or `implemented` (code exists and matches).
* Inside a spec, **(leaning)** marks a proposal not yet agreed and **(unverified)** marks something needing a small spike. These follow [IMPLEMENTATION](../IMPLEMENTATION.md).
* A spec records what is decided and why. It does not repeat PHASES.
* Once a spec is `implemented`, the code and tests are the source of truth. Trim the spec to the rationale the code cannot carry, or delete it ([IMPLEMENTATION §12](../IMPLEMENTATION.md)).
* A spec for a ported script describes a **faithful port** as the default, and lists approved improvements in a closing section, "Breaking changes that will improve the script after port". Improvements are opt-in until a release deliberately makes one the default.
* Each spec ends with an **Open questions** section, which is the only place open items live.
