# Layers

**Status:** implemented. The horizontal layers `internal/` is built in, the rule each layer follows, and what lint enforces. [CONFIG_ENGINE](CONFIG_ENGINE.md) is the config engine that sits in the lowest layer. The package-by-package layout is [PROJECT_LAYOUT](../specs/PROJECT_LAYOUT.md).

## The layers

Imports point down, never up:

```
cmd
 └─ cli (internal/cli/*)        ui (internal/ui/*)
     └─ domain (ship, bump, chain, chainload, workspace, appconfig, tagscheme,
        calver, registry, releasetag, steprun, lifecycle, bundle, bundlestore, bundlefetch, task/*)
         └─ gero (internal/gero/*)
             └─ apis (internal/apis/meta, internal/apis/lines, internal/apis/v1alpha2)
                 └─ platform (internal/platform/*)
```

| Layer | Holds | May import |
|---|---|---|
| `platform` | generic packages with no knowledge of gero, projects, releases or lines, that another tool could use as they are: clients, exec, YAML editing, version parsing, the template syntax, the config engine | platform and third-party packages |
| `apis` | the versioned config contract: the envelope, the kinds of each apiVersion, their defaults and rules. It depends on nothing gero-specific | platform and apis |
| `gero` | gero's own infrastructure, which knows it is gero but not the config model: the CLI settings, who commits are attributed to, the work directory of a run, the self-update and the proof a release is gero's, the shell integration, the texts and trailer of its commits and PRs | platform, apis and gero |
| domain | what the tool knows about projects, releases and shipping | platform, apis, gero and domain |
| `cli`, `ui` | one package per command, and the output seam | everything below; commands do not import each other except the shared helpers |
| `cmd` | the cobra shell | `internal/cli`, `internal/gero/clicfg`, `internal/gero/errs` |

**Where a package goes.** Ask what it would need to know. A package that knows nothing of gero is `platform`. One that knows gero's own conventions (its trailer, its bot, its branch names, how it edits a checkout) but nothing of projects, streams or holdings is `gero`. One that reads the config model is domain. `apis` is only the contract: what a config author writes, and the rules that say whether it is valid.

`lifecycle` is the SUSE lifecycle parser, kept for reading Rancher's lifecycle dates. `cmd/version` stays because the release build sets its variables by path.

## What lint enforces

* **Layer rules:** platform imports only platform; apis imports only platform and apis; gero imports only platform, apis and gero; domain does not import `cli` or `ui`; a command does not import another command except the shared helpers
* **Doors:** one home each for exec, go-git, YAML, the HTML parser, lipgloss, readline, `x/term`, go-gh and go-github, and one door to the application config
* **One reader of the model's documents:** only `internal/chain` reads the `Doc` of a Project, Stream, Artifact, Holding or Edit; everything else asks the type ([CHAIN_MODEL](CHAIN_MODEL.md)). Tests are held to it too
* Each rule is checked by a throwaway illegal import that must fail lint
