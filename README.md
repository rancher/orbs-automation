# ORBS Automation

Release and process automation for the Rancher ORBS team.

Many of our repos keep their own copies of similar release scripts, and the copies drift apart. This repo is the single, tested home for that tooling.

> **Status: early.** Most of the command tree is planned and does nothing yet. See [PHASES](docs/PHASES.md) for what comes when.

## What's here

- **`gero`**, the team CLI. It sits next to `gh`, reuses your `gh` login or `GH_TOKEN`, and works on a workstation or in CI.
- **Central config** (`config/`), the description of each repository we automate: release lines, tag rules and pull-request conventions.
- **Release tracking issues** (planned), kept up to date automatically so everyone can see where a release stands.

## Try it

```sh
go build -o gero .
./gero doctor    # checks your setup and says how to fix what is wrong
./gero --help
```

From a checkout of this repo `gero` finds `./config` itself. Elsewhere, see [CLI_CONFIG](docs/specs/CLI_CONFIG.md) for the user file.

## Read more

- [VISION](docs/VISION.md): why this exists and where it is going
- [Docs](docs/README.md): the architecture, phases and specs
- [Contributing workflow](docs/ai/red-green-commits.md): how we write code and tests here
