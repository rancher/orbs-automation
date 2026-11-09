# Images exist

**Status:** implemented.

Port of `validate_image_exists` from the `push-to-rancher` bash scripts: before a release is pushed downstream, check that the images it names are published where they must be. The command is `gero repo images exist`. The same check is the artifact precondition of a downstream bump, which `gero ship` runs once per release ([PUSH_TO_REPO](PUSH_TO_REPO.md)), so the logic is a library, `internal/registry`, and the command is a thin wrapper over it.

## The command

`gero repo images exist [name|owner/name] <tag>`

* The Project is found as for `gero repo info` ([WORKSPACE](WORKSPACE.md)): named, or the repository you are in. One argument is the tag, two are the project and the tag. The tag is used as given (`v0.5.3`).
* Read-only, and it needs the network. `--json` is supported.
* The text report is a title line (`<project> <tag> (stable|prerelease)`), a table with one row per image and registry (`IMAGE`, `REGISTRY`, `STATUS`, `REFERENCE`: the registry's label, the full reference, and `found`, `not found` or `unreachable` with the reason), the warnings, and a closing line. It is styled by the terminal of the settings ([CLI_OUTPUT](CLI_OUTPUT.md#terminal-capabilities)). `--json` is the one machine format and is never styled, and `--output env` is refused.
* Exit 0 when every required image is found. Exit 1 when any is not found or could not be reached. Usage and config errors should be 2, which waits for the exit-code helper ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)). The rows are printed before a non-zero exit.

`not found` and `unreachable` are different results: a missing image means the publish has not happened, and an unreachable registry means nothing is known. Both fail the run, and the report says which.

## What is checked

The script's rule, kept as it is: Docker Hub and Prime staging always, and Prime production only for a stable tag. A stable tag has no prerelease suffix, decided by `internal/platform/semver.Classify`, which is the script's own check ([CHECK_SEMVER](CHECK_SEMVER.md)).

### Go modules

A release is also checked for the Go modules it publishes: the `gomod` artifacts of the streams the tag is a release of (a stream whose `tagPattern` matches it, and an artifact whose `lines` include the tag's line). A module has a row after the images, with the registry `goproxy`, the label `Go module proxy` and the reference `module@version`, where the version is the tag without the prefix of its stream (`helmtools/v0.1.2` is looked for as `v0.1.2`). It is a plain HTTP request to the public Go module proxy for the version's `.info` (`internal/platform/goproxy`): `found` when the proxy has that version, `not found` for a 404 or a 410, and `unreachable` for anything else, an answer that is not that version included. The proxy fetches a tag it has not seen the first time it is asked, so asking is also what makes a new tag available. A prerelease is looked for too, since the proxy has them. A project whose check has no way to ask about modules shows the row as `skipped`. `gero ship` runs the same check, so a release the proxy cannot serve blocks a bump that `go mod tidy` would fail on. The proxy is not configurable yet: `Checker.BaseURL` is the place for another.

## Config

The images a project publishes are `Artifact` documents of kind `image` on its streams ([CONFIG_SCHEMA](../design/CONFIG_SCHEMA.md#artifact)):

```yaml
kind: Artifact
name: scc-operator-image
refs: {stream: scc-operator}
spec:
  kind: image
  name: rancher/scc-operator
  registries: [dockerhub, prime-staging, prime-production]   # optional
---
kind: Artifact
name: rancher-server-image
refs: {stream: rancher}
spec: {kind: image, name: rancher/rancher-server, lines: {since: v2.16}}
```

* There is no implicit image: an artifact is written for each thing a release produces, and a project with none has nothing to check ([CHAIN_MODEL](../design/CHAIN_MODEL.md)).
* `registries` narrows an image to some of the registries. Without it an image is checked in Docker Hub and Prime staging, and in Prime production for a stable tag (`Artifact.Registries`).
* `lines` is a range of Rancher lines (`since`, `until`), and the image is expected only for a tag in those lines. The tag's line is found from the project's `branches.lines` and the series of its streams, and lines compare by their numbers (`v2.16` is newer than `v2.15`). An image the tag is out of range for is not checked and is not in the report, since a row for it would only take up space. If that leaves nothing to check, the report says no image is expected for the tag.
* When the tag is in none of the project's lines, such as a tag of a line not in config yet, there is nothing to compare, so every image is checked in every registry, and the report carries a warning saying why. It is a warning and not an error because the check is still a useful one-off answer to "does this tag exist".
* Registries are named by role: `dockerhub`, `prime-staging` and `prime-production`. What a role means is built into `gero`, not config: its host, and that production applies only to stable tags. The hosts are `docker.io` (read through `registry-1.docker.io`), `stgregistry.suse.com` and `registry.rancher.com`. A role is a constant until there is a second registry or an auth need, which is the point to make it a document of its own. An unknown role is a config error.

## The registry client

It is `internal/platform/ociregistry`, which knows nothing of which registries gero uses; `internal/registry` has the three roles (their hosts and when each applies) and the report.

Standard `net/http`, no module, and no `docker` binary: `docker manifest inspect` is a manifest request, so the client makes one. It sends `HEAD /v2/<name>/manifests/<tag>` with the `Accept` types for image manifests and indexes. On a 401 with a bearer challenge it asks the named realm for an anonymous token and retries once. A 200 is found, a 404 is not found, and anything else, including a 401 after the retry, is unreachable with the status.

Reads are anonymous in every registry the script checks, so no credentials are used. Authentication for publishing stays in Vault and in the release workflow, never here. A registry that needs a login to read is reported as unreachable, not worked around.

Tests run the client against an `httptest` registry, including the token challenge, and the command against a fake client ([TESTING](TESTING.md)).

## Not built yet

None.

## Open questions

* Whether a registry that ignores `HEAD` needs a `GET` fallback. The three here are expected to answer `HEAD`.
* Whether the check should compare digests across registries, which the script does not do.
* Whether a mirror's timeout and retry policy needs to be a setting.
