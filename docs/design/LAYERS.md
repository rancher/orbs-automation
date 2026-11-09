# Layers

**Status:** implemented. The horizontal layers `internal/` is built in, the rule each layer follows, and what lint enforces. [CONFIG_ENGINE](CONFIG_ENGINE.md) is the config engine that sits in the lowest layer. The package-by-package layout is [PROJECT_LAYOUT](../specs/PROJECT_LAYOUT.md).

## The layers

Imports point down, never up:

```
cmd
 └─ cli (internal/cli/*)        ui (internal/ui/*)
     └─ domain (ship, bump, chain, workspace, tagscheme, calver, registry,
        releasetag, steprun, task/*, lifecycle, appconfig)
         └─ apis (internal/apis/meta, internal/apis/lines, internal/apis/v1alpha2)
             └─ platform (internal/platform/*)
```

| Layer | Holds | May import |
|---|---|---|
| `platform` | low-level packages with no knowledge of projects, releases or lines: clients, exec, YAML editing, version parsing, templates, the config engine, the CLI settings loader | platform and third-party packages |
| `apis` | the versioned config contract: the envelope, the kinds of each apiVersion, their defaults and rules | platform and apis |
| domain | what the tool knows about projects, releases and shipping | platform, apis and domain |
| `cli`, `ui` | one package per command, and the output seam | everything below; commands do not import each other except the shared helpers |
| `cmd` | the cobra shell | `internal/cli`, `platform/clicfg`, `platform/errs` |

`lifecycle` is the SUSE lifecycle parser, kept for reading Rancher's lifecycle dates. `cmd/version` stays because the release build sets its variables by path.

## What lint enforces

* **Layer rules:** platform imports only platform; apis imports only platform and apis; domain does not import `cli` or `ui`; a command does not import another command except the shared helpers
* **Doors:** one home each for exec, go-git, YAML, the HTML parser, lipgloss, readline, `x/term`, go-gh and go-github, and one door to the application config
* Each rule is checked by a throwaway illegal import that must fail lint
