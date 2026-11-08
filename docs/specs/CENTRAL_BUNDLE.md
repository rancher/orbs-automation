# Central Bundle

**Status:** implemented for the config chunk: the release publishes the bundle and `gero` reads it. The templates and the generated chunk are not built.

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

`tools/compilebundle` is a developer tool under `tools/`, like `tools/genschema`: not part of `gero`, importing its packages. It writes `gero-config-bundle.yaml` into the directory named by `--out`. Run it from the repository root:

```
go run ./tools/compilebundle --out .bundle --version v0.5.0 --commit "$(git rev-parse HEAD)" --date "$(git show -s --format=%cI HEAD)"
```

`--config` (default `config`) is the directory to bundle and `--out` (default `dist`) where to write it. The version, commit and date go in the header and are flags, never read from git or the clock, so the same input gives the same bytes. The tool loads the directory with `chainload.Load`, as every command does, and fails on invalid or empty config, so a bad bundle is never produced. It then loads the body of the bundle back, through an in-memory file system as a reader will, and fails unless it gives the same projects, holdings and lines. It writes nothing on failure. A test compiles the real `config/`, so a bad edit fails `go test ./...`.

## Publishing

The bundle ships with each `gero` release, as the asset `gero-config-bundle.yaml` of the release of `rancher/orbs-automation`, listed in the release's `checksums.txt`. GoReleaser does all of it, in one run ([.goreleaser.yaml](../../.goreleaser.yaml)):

* A `before` hook runs the tool with `--out .bundle` and the release's `{{ .Tag }}`, `{{ .FullCommit }}` and `{{ .CommitDate }}`. The output is outside `dist/`, which `--clean` empties, and `/.bundle` is git-ignored so the tree stays clean for GoReleaser.
* `checksum.extra_files` puts the bundle in `checksums.txt`, and `release.extra_files` uploads it with the archives to the draft release.

[release.yaml](../../.github/workflows/release.yaml) then does what GoReleaser cannot:

* The provenance attestation covers the bundle, with the archives and `checksums.txt`.
* Before the draft is published, a step fails the release unless the bundle is on it, in `checksums.txt`, and the uploaded copy has the hash `checksums.txt` lists.

[ci.yml](../../.github/workflows/ci.yml) runs `goreleaser check` and a snapshot release on every pull request, and checks that the bundle is in the snapshot's `checksums.txt`, so a change to the wiring fails before a tag does.

What this does not do: the checksums come from the same release as the bundle, so a reader that verifies them knows the download is intact, not who made it. Verifying the attestation is not built.

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

## What publishing looks like

1. A change to `config/` is a pull request to this repo. `go test ./...` compiles the real config, so one that does not validate fails there.
2. It merges, and a release is tagged (`v0.5.0`). `release.yaml` runs GoReleaser, whose hook compiles the bundle with that tag and commit.
3. The draft release holds the archives, `checksums.txt` and `gero-config-bundle.yaml`, which the checksums list. The bundle is attested with the archives, and the workflow refuses to publish a release where it is missing or does not match.
4. The release is published. A workstation with no local config finds it on its next run: when its cached copy is over a day old it asks for the newest stable release, downloads the bundle only if the release is a different one, checks it against `checksums.txt`, and replaces the cache. Within a day of a release every such workstation is on it, and a failed refresh keeps the old copy and says so.

A change to config therefore reaches workstations with a release, not with a merge. A job that needs a change sooner uses a checkout and `GERO_CENTRAL_CONFIG_DIR`.

## Built

* `internal/bundle`: the format, `Compile` (for the release) and `Parse` (for the reader), which share one definition and check each other.
* `internal/platform/assetfetch`: the one place a release asset is downloaded and verified, over `ghreleases`. An asset with no published checksum, or one that does not match, is never returned.
* `internal/bundlestore`: the cache (`$XDG_CACHE_HOME/gero/`, else `~/.cache/gero/`, which replaces its bundle and record together and treats damage as empty) and the resolver, which applies the release rules above, refreshes at most daily, and falls back to the cache saying why.
* `internal/workspace`: the fifth source, `ConfigSource`, and the rule that a local source always wins. `gero config show` and `gero doctor` report the source. Doctor is offline unless `--online`, and then reads the cache only.
* `bundletest` and `fakehub`: imagined releases for tests, with a tampered asset, a missing checksum, a rate limit and a bundle of a newer format, all without a network.

## Not built yet

* Verifying the attestation of the release. The checksums come from the same release as the bundle, so a download is known to be intact, not to be the maintainers'.
* The newer of a local copy and the bundle: a local source always wins.
* Conditional requests (`ghreleases` has none), so the daily refresh is by the age of the cache alone.
* The templates and generated chunks, a way to name a release to use, and a refresh interval setting.

## Open questions

* What `ghreleases` does about the REST API's unauthenticated rate limit (60 requests an hour per IP, shared by CI runners and office networks), and whether it verifies checksums itself or the shared package does. If it uses the API, the daily refresh limit matters more.
* Whether the central repo's setting is still `centralConfigDir`, or becomes a directory for the whole central repo, now that the bundle holds templates as well as config ([CLI_CONFIG](CLI_CONFIG.md)).
* How `gero ci:*` commands in other repos get config. Today the bundle is never used in CI, which sets `GERO_CENTRAL_CONFIG_DIR` to a checkout of the central repo. Whether a job without a checkout should use the bundle of the installed release is not decided.
* Whether the refresh interval is a setting.
