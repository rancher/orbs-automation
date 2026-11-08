# Central Bundle

**Status:** draft. Nothing here is built.

How the central repo's shared content ships as one file with each `gero` release, and how the CLI uses that file when no local checkout is available. The content is the application config ([APP_CONFIG](APP_CONFIG.md)) and the workflow templates for `gero workflows sync` ([ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md)). How a local directory is found is in [CLI_CONFIG](CLI_CONFIG.md#locating-the-central-application-config).

## Why

Today a command needs the central `config/` directory on disk, so every workstation and every job needs a clone or checkout of the central repo. The release of `gero` happens in a public repo, so the same release can carry a compiled copy of what the CLI needs from that repo, and the CLI can fetch it with no clone and no token.

A local source always wins. The bundle is the fallback for when none is configured, never an override.

## The bundle

One multi-document YAML file. It is chunks of the same documents the repo holds, not a new schema: loading the config chunk yields the same `config.Config` as loading the `config/` directory.

| Chunk | Meaning |
|---|---|
| Header | The first document. Bundle format version, the `gero` release tag it was compiled for, the source commit, the compile time |
| Config | The `config/` documents as written, in a fixed order, so compiling the same tree twice gives the same bytes |
| Templates | The workflow templates, as documents carrying their path and content |
| Generated | Documents of a kind that marks them as compile-time output, always after the others. Never mixed into the config documents, so a reader can tell hand-written content from generated data |

The seam for config is `config.Load`, which takes an `fs.FS`: the config chunk is read into the same loader through an in-memory file system, so validation and defaults are shared and commands do not know where the config came from. Templates get the same treatment when `workflows sync` exists.

**Generated is a place, not a plan.** No generated document exists yet. The expected use is caches that save work at run time, for example API results that rarely change. Each one needs its own spec before it is added, including who refreshes it and what happens when it is stale.

## Compile command

`gero ci:compile-bundle` is CI-only ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)). It reads the central repo's directories, validates the config as `gero config validate` does, runs any generating steps, and writes the bundle to a path given as a flag. It fails on invalid input, so a bad bundle is never produced.

The release job of the central repo runs it and attaches the bundle to the GitHub release next to the `gero` binaries, with a checksum file.

## Releases, fetching and verifying

Every download of a release asset, the bundle or anything else, is verified against its published checksum before it is used or written to a cache. A mismatch is an error and the previous cached copy stays. The one implementation of "download a release asset and verify it" lives in one package and every caller uses it; it is built on [`ghreleases`](https://github.com/mallardduck/ghreleases), approved in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md).

**Which release.**

* Only **stable** releases are candidates for "latest". A prerelease is never picked because it is the newest.
* A stable `gero` uses the newest stable release's bundle.
* A prerelease `gero` defaults to the bundle of its own release, since the two were made together, but prefers the newest stable bundle when that is newer than itself. If the loader cannot read that newer bundle (see Compatibility), it falls back to its own.
* A flag naming a release tag overrides all of this, for reproducing an old run **(leaning)**.

**Caching.**

| Rule | Detail |
|---|---|
| Where | `$XDG_CACHE_HOME/gero/`, else `~/.cache/gero/`. A cache, not config: it can be deleted at any time |
| Refresh | At most once a day, using a conditional request where the endpoint supports it, so an unchanged release is cheap |
| Offline | When the fetch fails and a cached copy exists, the cached copy is used and the command says so, with its release tag and age |
| Nothing available | No local source, no cache and no network fails with the same message that lists the places config is looked for |

## Source precedence

The bundle is one more source, after the local ones in [CLI_CONFIG](CLI_CONFIG.md#locating-the-central-application-config):

1. `--central-config-dir`
2. `GERO_CENTRAL_CONFIG_DIR`
3. `centralConfigDir` in the user file
4. `./config`, when the working directory is a checkout of the central repo
5. The cached bundle, refreshed from the release chosen above

## Visibility and compatibility

* **Say which source was used.** `gero config show` and `gero doctor` report the source of the config, and for a bundle its release tag and age. A stale cache must never stand in for local config without saying so.
* **Check versions on load.** The loader checks the bundle format version and each document's `apiVersion`, and fails with a message naming both versions when this `gero` cannot read the bundle. A CLI older than its newest bundle is expected, so this check is the contract, not an edge case.

## Not built yet

Everything: the bundle format and loader, `ci:compile-bundle`, the release job step, the shared download-and-verify package, the cache, the fifth source in precedence, and the source line in `config show` and `doctor`.

## Open questions

* The name of the release asset and of the checksum file, and which repo the CLI fetches from (a setting, or fixed in the binary).
* What `ghreleases` does about the REST API's unauthenticated rate limit (60 requests an hour per IP, shared by CI runners and office networks), and whether it verifies checksums itself or the shared package does. If it uses the API, the daily refresh limit matters more.
* Whether the central repo's setting is still `centralConfigDir`, or becomes a directory for the whole central repo, now that the bundle holds templates as well as config ([CLI_CONFIG](CLI_CONFIG.md)).
* How `gero ci:*` commands in other repos get the bundle: use the one the installed `gero` release implies, fetch the latest stable, or check the central repo out. Not decided. The central repo's own CI uses its checkout and does not need the bundle.
* Whether the refresh interval is a setting.
