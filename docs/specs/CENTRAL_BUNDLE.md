# Central Bundle

**Status:** implemented for the config chunk: the release publishes the bundle and `gero` reads it. The templates and the generated chunk are not built.

How the central repo's shared content ships as one file with each `gero` release, and how the CLI uses that file when no local checkout is available. The content is the application config ([APP_CONFIG](APP_CONFIG.md)) and the workflow templates for `gero workflows sync` ([ARCHITECTURE_DESIGN](../ARCHITECTURE_DESIGN.md)). How a local directory is found is in [CLI_CONFIG](CLI_CONFIG.md#locating-the-central-application-config).

## Why

A command that reads only `config/` from disk would need every workstation and every job to have a clone or checkout of the central repo. The release of `gero` happens in a public repo, so the same release can carry a compiled copy of what the CLI needs from that repo, and the CLI can fetch it with no clone and no token.

The bundle is the first layer: gero assumes the user has no clone of the central repo. A local directory is a development mode on top of it, and is read only when one is present (with `configSource: auto`) or asked for (`dir`). `configSource: bundle`, in the user file or as `--config-source=bundle`, reads the bundle whatever is set locally, and `--bundle-file` reads one downloaded bundle ([CLI_CONFIG](CLI_CONFIG.md#locating-the-central-application-config)).

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
* A step signs `checksums.txt`, each archive and the config bundle with keyless Sigstore, uploads the signatures to the draft and fails the release if one is missing. `gero doctor self-update` verifies the manifest and the archives ([SELF_UPDATE](SELF_UPDATE.md)), and the bundle fetch verifies the manifest and the bundle. The bundle is listed in `checksums.txt`, so the manifest's signature covers it too, and `gero-config-bundle.yaml.sigstore.json` lets it be verified on its own.
* Before the draft is published, a step fails the release unless the bundle is on it, in `checksums.txt`, and the uploaded copy has the hash `checksums.txt` lists.

[ci.yml](../../.github/workflows/ci.yml) runs `goreleaser check` and a snapshot release on every pull request, and checks that the bundle is in the snapshot's `checksums.txt`, so a change to the wiring fails before a tag does.

The checksums come from the same release as the bundle, so on their own they prove the download is intact, not who made it. The signatures close that gap, and fetching the bundle verifies them (below). The provenance attestation is still not checked by `gero`; it is for people who verify by hand with `gh attestation verify`.

## Releases, fetching and verifying

Every download of a release asset, the bundle or anything else, is verified before it is used or written to a cache, in two ways. It must match its published checksum, and the signatures must verify: `checksums.txt.sigstore.json` over the manifest and `<asset>.sigstore.json` over the asset itself, keyless Sigstore bundles by the pinned identity, the release workflow of this repository on a version tag, the same proof self-update asks for ([SELF_UPDATE](SELF_UPDATE.md#what-proves-the-release-is-ours)). A mismatch, a missing signature or a signature by anyone else is an error and fails closed: nothing falls back to the checksum alone, and the previous cached copy stays. A release made before it was signed is therefore refused, and the cache is used instead. The Sigstore trust root is fetched over TUF when the first signature is checked, so a download needs more than GitHub; a bundle already cached is used without it. The one implementation of "download a release asset and verify it" lives in one package and every caller uses it; it is built on [`ghreleases`](https://github.com/mallardduck/ghreleases), approved in [DEPENDENCY_POLICY](DEPENDENCY_POLICY.md).

**Which release.**

* `bundleChannel` decides whether prereleases are candidates: `stable`, `prerelease`, or `auto`, the default, which follows the running `gero` as self-update does. A stable `gero` considers only stable releases, and a prerelease is never picked because it is the newest. A prerelease `gero` considers all of them. `stable` makes any `gero` take only stable releases, with no floor at its own release; `prerelease` makes any `gero` consider all of them, a pre-release one keeping its own as the floor and the fallback, a stable one falling back to the newest stable when the newest bundle is unreadable. A cached bundle is used without a request only if its release fits the channel, so changing it takes effect on the next run.
* A stable `gero`, or a build that is not a release, uses the newest stable release's bundle.
* A prerelease `gero` uses the bundle of the newest release by version, a prerelease or not, so `v0.6.0-rc.2` takes the bundle of `v0.6.0-rc.3` or of `v0.6.0`, and works when no stable release exists yet. Its own release is the floor, since the two were made together. If the loader cannot read the newer bundle (see Compatibility), it falls back to its own.

**Caching.**

| Rule | Detail |
|---|---|
| Where | `$XDG_CACHE_HOME/gero/`, else `~/.cache/gero/`. A cache, not config: it can be deleted at any time |
| Refresh | At most once a day, by the age of the cache alone, since `ghreleases` makes no conditional requests. The bundle is downloaded only when the newest release is a different one |
| Offline | When the fetch fails and a cached copy exists, the cached copy is used and the command says so, with its release tag and age |
| Nothing available | No local source, no cache and no network fails with the same message that lists the places config is looked for |

## Source precedence

With `configSource: auto`, the default, a local source that is present is read and the bundle is the fallback. In order ([CLI_CONFIG](CLI_CONFIG.md#locating-the-central-application-config)):

0. `--bundle-file`, a bundle file given by hand, which ends the search
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
4. The release is published. A workstation with no local config finds it on its next run: when its cached copy is over a day old it asks for the newest stable release, downloads the bundle only if the release is a different one, checks it against `checksums.txt` and its signatures, and replaces the cache. Within a day of a release every such workstation is on it, and a failed refresh keeps the old copy and says so.

A change to config therefore reaches workstations with a release, not with a merge. A job that needs a change sooner uses a checkout and `GERO_CENTRAL_CONFIG_DIR`.

## Not built yet

* The newer of a local copy and the bundle: in `auto` a local source wins, and `--config-source` picks one or the other.
* A refresh interval setting. Reproducing an old run needs no more: download that release's bundle and give it with `--bundle-file`.
* The templates and generated chunks.

## Open questions

* What `ghreleases` does about the REST API's unauthenticated rate limit (60 requests an hour per IP, shared by CI runners and office networks). If it uses the API, the daily refresh limit matters more.
* Whether `centralConfigDir`, which names the central repo's `config/` directory, should name the whole central repo once the bundle holds templates as well as config ([CLI_CONFIG](CLI_CONFIG.md)).
* How `gero ci:*` commands in other repos get config. Today the bundle is never used in CI, which sets `GERO_CENTRAL_CONFIG_DIR` to a checkout of the central repo. Whether a job without a checkout should use the bundle of the installed release is not decided.
* Whether the refresh interval is a setting.
