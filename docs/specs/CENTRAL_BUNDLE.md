# Central Bundle

**Status:** draft. The producer is built, `tools/compilebundle`, which writes the config chunk; the reader in `gero`, the release wiring, the templates and the generated chunk are not.

How the central repo's shared content ships as one file with each `gero` release, and how the CLI uses that file when no local checkout is available. The content is the application config ([APP_CONFIG](APP_CONFIG.md)) and the workflow templates for `gero workflows sync` ([ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md)). How a local directory is found is in [CLI_CONFIG](CLI_CONFIG.md#locating-the-central-application-config).

## Why

Today a command needs the central `config/` directory on disk, so every workstation and every job needs a clone or checkout of the central repo. The release of `gero` happens in a public repo, so the same release can carry a compiled copy of what the CLI needs from that repo, and the CLI can fetch it with no clone and no token.

A local source always wins. The bundle is the fallback for when none is configured, never an override.

## The bundle

One multi-document YAML file. It is chunks of the same documents the repo holds, not a new schema: loading the config chunk yields the same `config.Config` as loading the `config/` directory.

| Chunk | Meaning |
|---|---|
| Header | The first document: `apiVersion: gero-bundle/v1`, `kind: Bundle`, `formatVersion`, `geroVersion` (the release tag it was compiled for), `sourceCommit` and `compiledAt`. It is not a config kind and is not in `gero/v1alpha2`: the reader takes it off before the config |
| Config | The documents of each `config/` YAML file, in the order of the file names, each file after a `# source: <file>` comment. They are as written, except that the `# yaml-language-server` comment at the top of a file and a leading `---` are dropped: the comment only works as a file's first line, and the bundle is not edited, so it carries none. Compiling the same tree twice gives the same bytes |
| Templates | Later. The workflow templates, as documents carrying their path and content. Today the templates are compiled into the binary and the bundle holds none |
| Generated | Documents of a kind that marks them as compile-time output, always after the others. Never mixed into the config documents, so a reader can tell hand-written content from generated data |

The seam for config is `chainload.Load`, which takes an `fs.FS` (`appconfig.Load` under it): the config chunk is read into the same loader through an in-memory file system, so validation and defaults are shared and commands do not know where the config came from. Templates get the same treatment when `workflows sync` exists.

**Generated is a place, not a plan.** No generated document exists yet. The expected use is caches that save work at run time, for example API results that rarely change. Each one needs its own spec before it is added, including who refreshes it and what happens when it is stale.

## Compile command

`tools/compilebundle` is a developer tool under `tools/`, like `tools/genschema`: not part of `gero`, importing its packages. It runs at release time and writes `gero-config-bundle.yaml` into `dist/`, beside what GoReleaser writes. Run it from the repository root, after GoReleaser, which empties `dist/` when it starts:

```
go run ./tools/compilebundle --version v0.5.0 --commit "$(git rev-parse HEAD)" --date "$(git show -s --format=%cI HEAD)"
```

`--config` (default `config`) is the directory to bundle and `--out` (default `dist`) where to write it. The version, commit and date go in the header and are flags, never read from git or the clock, so the same input gives the same bytes. The tool loads the directory with `chainload.Load`, as every command does, and fails on invalid or empty config, so a bad bundle is never produced. It then loads the body of the bundle back, through an in-memory file system as a reader will, and fails unless it gives the same projects, holdings and lines. It writes nothing on failure. A test compiles the real `config/`, so a bad edit fails `go test ./...`.

Not wired yet, for when the release is set up:

* Run it in the release job after `goreleaser release --clean`, with the tag, `GITHUB_SHA` and the commit time.
* Add the bundle to `checksums.txt` and to the `subject-path` of the provenance attestation in `.github/workflows/release.yaml`.
* Attach it to the release with `gh release upload`, or `release.extra_files` of GoReleaser, before the release is published.

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
