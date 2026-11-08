# Layers

**Status:** working notes, 2026-11-07. The horizontal layers `internal/` is moving to, the rule each layer follows, and what lint enforces. [CONFIG_ENGINE](CONFIG_ENGINE.md) is the config engine that sits in the lowest layer. `docs/specs/PROJECT_LAYOUT.md` describes what exists today and is rewritten after the moves.

## The layers

Imports point down, never up:

```
cmd
 └─ cli (internal/cli/*)        ui (internal/ui/*)
     └─ domain (ship, bump, chain, workspace, tagscheme, calver, registry,
        releasetag, steprun, task/*, actor, lifecycle, appconfig)
         └─ apis (internal/apis/meta, internal/apis/v1alpha2)
             └─ platform (internal/platform/*)
```

| Layer | Holds | May import |
|---|---|---|
| `platform` | low-level packages with no knowledge of projects, releases or lines: clients, exec, YAML editing, version parsing, templates, the config engine, the CLI settings loader | platform and third-party packages |
| `apis` | the versioned config contract: the envelope, the kinds of each apiVersion, their defaults and rules | platform and apis |
| domain | what the tool knows about projects, releases and shipping | platform, apis and domain |
| `cli`, `ui` | one package per command, and the output seam | everything below; commands do not import each other except the shared helpers |
| `cmd` | the cobra shell | `internal/cli`, `platform/clicfg`, `platform/errs` |

## Where each package goes

| Today | Layer and path |
|---|---|
| execx, errs, yamledit, semver, shellenv, workdir, clicfg | `internal/platform/<same>` |
| git, git/gitcli, git/repo, git/repo/repotest | `internal/platform/git/...` |
| github, github/pulls, ghcli | `internal/platform/github`, `.../github/pulls`, `.../ghcli` |
| ship/templates | `internal/platform/templates` |
| (new) | `internal/platform/config`, the engine |
| git/releasetag | `internal/releasetag` (tag policy, so domain) |
| calver, tagscheme, registry, bump, chain, chainload, appconfig, steprun, ship, workspace, task/*, actor, lifecycle | stay |
| apis/meta, apis/v1alpha2 | stay |
| config, config/loader | stay until the cutover from `gero/v1alpha1`, then are deleted |
| ui, ui/style, ui/editor, cli/*, cmd/* | stay |

`lifecycle` is unused today but is reserved for the SUSE lifecycle parser, so it stays. `cmd/version` stays because the release build sets its variables by path.

## What lint enforces

* **Layer rules (new):** platform imports only platform; apis imports only platform and apis; domain does not import `cli` or `ui`; a command does not import another command except the shared helpers
* **Doors (existing):** one home each for exec, go-git, YAML, the HTML parser, lipgloss, readline, `x/term`, go-gh and go-github, and one door to the application config
* Each rule is checked by a throwaway illegal import that must fail lint

## How the moves are made

One package per commit, bottom-up from the packages with no internal imports: `git mv`, import rewrite, the matching rule edit and the docs for that package, in the same commit. Each commit builds and passes tests.
