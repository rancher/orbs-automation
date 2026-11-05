# Images exist

**Status:** implemented.

Port of `validate_image_exists` from the `push-to-rancher` bash scripts: before a release is pushed downstream, check that the images it names are published where they must be. The command is `gero repo images exist`. The same check is the artifact precondition of a downstream bump ([PUSH_TO_REPO](PUSH_TO_REPO.md)), so the logic is a library, `internal/registry`, and the command is a thin wrapper over it.

## The command

`gero repo images exist [name|owner/name] <tag>`

* The Project is found as for `gero repo info` ([WORKSPACE](WORKSPACE.md)): named, or the repository you are in. One argument is the tag, two are the project and the tag. The tag is used as given (`v0.5.3`).
* Read-only, and it needs the network. `--json` is supported.
* One row per image and registry: the registry's label, the full reference, and `found`, `not found` or `unreachable` with the reason.
* Exit 0 when every required image is found. Exit 1 when any is not found or could not be reached. Usage and config errors are 2 ([CLI_FRAMEWORK](CLI_FRAMEWORK.md)). The rows are printed before a non-zero exit.

`not found` and `unreachable` are different results: a missing image means the publish has not happened, and an unreachable registry means nothing is known. Both fail the run, and the report says which.

## What is checked

The script's rule, kept as it is: Docker Hub and Prime staging always, and Prime production only for a stable tag. A stable tag has no prerelease suffix, decided by `internal/semver.Classify`, which is the script's own check ([CHECK_SEMVER](CHECK_SEMVER.md)).

## Config

A Project may list the images it publishes:

```yaml
spec:
  images:                       # optional
    - name: rancher/scc-operator
      registries: [dockerhub, prime-staging, prime-production]
    - name: rancher/rancher-server
      since: v2.16                # only for tags in this line and newer
```

* With no `images`, the Project publishes one image named `repo.name` to every registry. That is the common case and needs no config. The effective list comes from a getter on the Project, and the field stays as written ([WORKSPACE](WORKSPACE.md#raw-and-effective-values)).
* An entry's `name` overrides the default, and several entries track several images. `registries` narrows an image to some of the registries, and defaults to all of them.
* `since` is a Rancher line ID, and the image is expected only for a tag in that line or a newer one. The tag's line is found from the Project's `release.lines` and their series ([WORKSPACE](WORKSPACE.md#raw-and-effective-values)), and lines compare by their numbers (`v2.16` is newer than `v2.15`), so only the Project is needed. An image the tag is too old for is reported `skipped (before v2.16)` and does not fail the run. `since` must be a line in `RancherLines`, and it is an error on a Project with no `release.lines`.
* When the tag is in none of the Project's lines, such as a tag of a line not in config yet, there is nothing to compare, so the check is a plain one: every image is checked in every registry, with no `since` rule applied, and the report carries a warning saying why. It is a warning and not an error because the check is still a useful one-off answer to "does this tag exist".
* Registries are named by role: `dockerhub`, `prime-staging` and `prime-production`. What a role means is built into `gero`, not config: its host, and that production applies only to stable tags. The hosts are `docker.io` (read through `registry-1.docker.io`), `stgregistry.suse.com` and `registry.rancher.com`. A role is a constant until there is a second registry or an auth need, which is the point to make it a document of its own ([APP_CONFIG](APP_CONFIG.md)). An unknown role is a config error.

## The registry client

Standard `net/http`, no module, and no `docker` binary: `docker manifest inspect` is a manifest request, so the client makes one. It sends `HEAD /v2/<name>/manifests/<tag>` with the `Accept` types for image manifests and indexes. On a 401 with a bearer challenge it asks the named realm for an anonymous token and retries once. A 200 is found, a 404 is not found, and anything else, including a 401 after the retry, is unreachable with the status.

Reads are anonymous in every registry the script checks, so no credentials are used. Authentication for publishing stays in Vault and in the release workflow, never here. A registry that needs a login to read is reported as unreachable, not worked around.

Tests run the client against an `httptest` registry, including the token challenge, and the command against a fake client ([TESTING](TESTING.md)).

## Not built yet

* `since`, and the warning for a tag in no line.
* `until`, to stop expecting an image after some line. Nothing needs it yet, so it is not designed; an image that is dropped is the case that would.
* The downstream bump does not call `internal/registry` as its artifact precondition yet, because the bump engine does not exist ([PUSH_TO_REPO](PUSH_TO_REPO.md)).

## Open questions

* Whether a registry that ignores `HEAD` needs a `GET` fallback. The three here are expected to answer `HEAD`.
* Whether the check should compare digests across registries, which the script does not do.
* Whether a mirror's timeout and retry policy needs to be a setting.
