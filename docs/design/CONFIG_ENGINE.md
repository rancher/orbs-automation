# Config Engine

**Status:** implemented, 2026-11-08. The engine in `internal/platform/config` and the providers built on it. Where it sits is in [LAYERS](LAYERS.md); the kinds it loads are in [CONFIG_SCHEMA](CONFIG_SCHEMA.md).

## What it does

The engine handles one object. It decodes strictly, fills safe defaults and validates, and reports every finding. It knows nothing about kinds, files or references.

1. **Decode:** read the YAML into the target, rejecting any field the target does not have
2. **Defaults:** `SetDefaults` fills the fields that have a default
3. **Validate:** `Validate` checks the object, reading the defaulted values

This is the order the provider runs for each document (decode, then defaults, then validate).

## Defaults are safe and independent

A default fills only a field that has a conservative value of its own, such as `layout: single`, `scheme: semver`, `at.format: yaml`, `holds.accept: stable`. A value that depends on something else is not filled: it stays as written and a getter gives the effective value (`managedBy` is `orbs` only if the repo is owned; a line's branch depends on its phase). The same "raw and effective" rule `docs/specs/WORKSPACE.md` states for today's config.

## Findings and warnings

* A **finding** is an error or a warning. It names the source (a file), the document (`Project "rancher"`), the path in the document and the message
* A **report** collects findings. `Err()` returns the errors joined (always a join, so a consumer that walks to a joined error keeps working); warnings are listed apart
* A lifecycle date, a circular holding and a line with an omitted `branch` after its development are warnings, never errors ([CONSTRAINTS](CONSTRAINTS.md))
* The CLI prints warnings and exits clean

## Validating across documents

One object cannot check references to others. A version package adds a set check that runs after every document is decoded and defaulted: unique names, references that resolve to the kind their key names, unique repo slugs, at most one final destination, at most one default-branch line for a project, series that do not overlap, and a holding whose reference matches what it holds. It is skipped when any document failed to decode, as today.

## Providers

| Provider | Does | Keeps |
|---|---|---|
| `platform/clicfg` | the CLI settings from flags, environment, the user file and defaults | precedence and where each value came from, which a single decode cannot know |
| `appconfig` | finds the files of a config directory, decodes each document by apiVersion and kind, applies defaults, validates each object and the set | one door: `workspace` is the only importer |
| `workspace` | the one door from settings to the world | the order: CLI settings first, then application config |

The envelope, the kind registry and the multi-document loop stay in `apis/meta`; for each document it calls the engine and continues past a bad one. A YAML syntax error stops the file.

## What stays as it is

* The old `internal/config` and its loader were deleted in the [cutover](CUTOVER.md); the engine is what loads the config
* The user's CLI config schema does not change
