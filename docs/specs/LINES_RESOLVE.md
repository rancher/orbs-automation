# LINES_RESOLVE

**Status:** implemented, with the parts under [Not built yet](#not-built-yet) open. `gero lines resolve`: the map of how the release lines of the projects connect.

The one shared line map ([IMPLEMENTATION](../IMPLEMENTATION.md#repo-shapes-and-downstream-bumps)) is in two places: the `RancherLines` document holds the facts of each Rancher line, and each project maps its own series and branch to line ids under `release.lines`. This command reads both and the edges between the projects, and joins them by line, so one screen says what `v2.15` is across the projects and what feeds what. It reads the central config and the clock and never the network; what each branch holds on the host is [REPO_STATUS](REPO_STATUS.md).

```
Release lines, through rancher/rancher

v2.15  ga  GA 2026-08-27, EOM 2027-02-27, EOL 2028-02-27  Kubernetes 1.34-1.36
  rancher/rancher  release/v2.15
    takes from
      scc-operator  any release                 build-yaml-bump (defaultSccOperatorImage)
      shell         0.8.x        release/v2.15  build-yaml-bump (defaultShellVersion)
    ships to
      -
    also on this line
      charts  release-v2.15
        takes from obteam (any release) via build-yaml-bump (obChart)

No gaps.
```

* **Lines are newest first**, ordered by their numbers (`v2.16` before `v2.9`), whatever order the document has them in. A line is shown with its phase from the clock (`development`, `ga`, `eom`, `eol`, as [APP_CONFIG](APP_CONFIG.md) derives it), the dates that are set and the Kubernetes range.
* **The hub** is the project the lines are connected through: the one the lines document is named for (`rancher`), else `rancher/rancher`, and `--hub` names another. Most things end in it, either directly or through another project (`rancher/charts`, whose charts reach it in an image), and the chain is not modeled in the config, so it is not needed to get a useful map.
* **Takes from** are the projects with an edge to the hub whose releases go to the hub's branch for the line, found with the routes `gero ship` ships down. A producer with its own lines is shown with its series and branch for the line; one with none (`universal`) goes to every release branch of the hub, so it is on every line the hub has a branch for, as `any release`. Each row says the update method and the key it updates.
* **Ships to** are the projects the hub's releases for the line go to, with the branches. A consumer with no branch for the line is left out.
* **Also on this line** is every other project that has the line but no direct link to the hub on it, with its own connections, so a chain through it shows. One hop from the hub is followed; the projects reached only through another are listed here with that other connection, and not followed further.
* **Gaps** are what does not connect, marked with `!` and counted at the end: a producer that follows lines and ships to the hub but has no such line, or has a line the hub does not have. A project that does not have a line the hub has, and ships to nothing on it, is not a gap.
* **Filters.** Lines named as arguments (`v2.15` or `2.15`) and `--phase` (one or several of `development`, `ga`, `eom`, `eol`) narrow the lines shown. An unknown line or phase is an error that says what there is. `--json` carries the same map as `lines`, each with its `hub`, `takesFrom`, `shipsTo`, `also` and `gaps`.

## Not built yet

* Following the chain beyond one hop, such as `ob-team-charts` into `charts` into `rancher` (`also` lists the project and its own links today; it does not connect them back to the hub).
* A line the lines document has no entry for, but a project has: validation rejects it when the config is loaded, so there is no gap for it here.
* The answer for one producer and one line, "where does shell's `v2.15` go", which is a different command and not this map.

## Open questions

* Whether the Rancher Assets path (charts to the assets image to rancher) is worth modeling as edges, so the chain through `rancher/charts` connects back to the hub and leaves `also`.
