# LINES_RESOLVE

**Status:** implemented, except the parts under [Not built yet](#not-built-yet). `gero lines resolve`: the map of how the release lines of the projects connect.

The one shared line map ([IMPLEMENTATION](../IMPLEMENTATION.md#repo-shapes-and-downstream-bumps)) is in two places: the `RancherLines` document holds the facts of each Rancher line, and each project maps its own series (in its streams) and branch (in `branches.lines`) to line ids. This command reads both and the holdings between the projects, and joins them by line, so one screen says what `v2.15` is across the projects and what feeds what. It reads the central config and the clock and never the network; what each branch holds on the host is [REPO_STATUS](REPO_STATUS.md).

```
Release lines, through rancher/rancher

v2.15  ga  GA 2026-08-27, EOM 2027-02-27, EOL 2028-02-27  Kubernetes 1.34-1.36
  rancher/rancher  release/v2.15  series 2.15.x
    takes from
      scc-operator  any release                 rancher-holds-scc-operator
      shell         0.8.x        release/v2.15  rancher-holds-shell
    also on this line
      backup-restore-operator  release/v11.x  series 11.x
        takes from kuberlr-kubectl (8.x) via bro-holds-kuberlr
        ships to rancher-charts (dev-v2.15) via charts-holds-bro-chart, charts-holds-bro-crd-chart
      rancher-charts  series 110.x
        takes from backup-restore-operator (11.x) via charts-holds-bro-chart, charts-holds-bro-crd-chart

No gaps.
```

* **Lines are newest first**, ordered by their numbers (`v2.16` before `v2.9`), whatever order the document has them in. A line is shown with its phase from the clock (`development`, `ga`, `eom`, `eol`, as [APP_CONFIG](APP_CONFIG.md) derives it), the dates that are set and the Kubernetes range.
* **The hub** is the project the lines are connected through: the one project marked `finalDestination: true` in the config ([APP_CONFIG](APP_CONFIG.md)), which is `rancher/rancher`, and `--hub` names another. Most things end in it, either directly or through another project (`rancher/charts`, whose charts reach it in an image), and the walk beyond one hop is not built, so it is not needed to get a useful map. The flag is a field of the config and not a project known by name, so the map has no special case in code; no flag and no `--hub` is an error that says so.
* **Takes from** are the projects that the hub holds a release of, whose releases go to the hub's branch for the line, found with the routes `gero ship` ships down. A producer with its own lines is shown with its series and branch for the line; one with none (`universal`) goes to every release branch of the hub, so it is on every line the hub has a branch for, as `any release`. Each row names the holdings that make the connection.
* **Ships to** are the projects the hub's releases for the line go to, with the branches, shown when the hub is named with `--hub` and is not the final destination. Nothing ships on from the final destination, so the section is not shown for it, whatever holdings others have of it, and a project that holds something of it is then just on the line. A consumer with no branch for the line is left out.
* **Also on this line** is every other project that has the line but no direct link to the hub on it, with its own connections, so a chain through it shows. One hop from the hub is followed; the projects reached only through another are listed here with that other connection, and not followed further.
* **Gaps** are what does not connect, marked with `!` and counted at the end: a producer that follows lines and ships to the hub but has no such line, or has a line the hub does not have. A project that does not have a line the hub has, and ships to nothing on it, is not a gap. Neither is a line that a project whose lines are in its tags does not list (`rancher-assets` lists `v2.16`): it lists the lines it is used for and only those are judged, so it is on `v2.16` as a source and absent from the others without a warning ([CHAIN_TRACING](CHAIN_TRACING.md)).
* **Filters.** Lines named as arguments (`v2.15` or `2.15`) and `--phase` (one or several of `development`, `ga`, `eom`, `eol`) narrow the lines shown. An unknown line or phase is an error that says what there is. `--json` carries the same map as `lines`, each with its `hub`, `takesFrom`, `shipsTo`, `also` and `gaps`.

## Not built yet

* Following the chain beyond one hop, such as `ob-team-charts` into `charts` into `rancher` (`also` lists the project and its own links today; it does not connect them back to the hub).
* A line the lines document has no entry for, but a project has: validation rejects it when the config is loaded, so there is no gap for it here.
* The answer for one producer and one line, "where does shell's `v2.15` go", which is a different command and not this map.

## Open questions

* How the chain through `rancher/charts` connects back to the hub and leaves `also`: the holdings of Assets in Rancher and of charts in Assets are in the config, and following them is the walk of [CHAIN_TRACING](CHAIN_TRACING.md), which is not built.
