# CONFIG_GRAPH

**Status:** implemented. `gero config resolve graph`: the whole application config drawn as a D2 diagram, or with the `mindmap` argument as a Mermaid mind map.

The config joins into one graph ([CHAIN_MODEL](../design/CHAIN_MODEL.md)), which the other commands read one question at a time. This command draws all of it at once, so the projects and the way releases flow between them are seen together. It reads the central config, and the clock only for the phase of a release line, and never the network, and writes D2 source to stdout ([CLI_OUTPUT](CLI_OUTPUT.md)); render it with `d2 graph.d2 graph.svg`.

* **The diagram flows down**, so a config with many projects grows tall and is read by scrolling, not wide.
* **Projects are the primary nodes**, containers of the streams they release and, inside each stream, the artifacts it produces. They are keyed by the repository's `owner/name` slug and labelled with the config name. Every project that is not external is drawn, whichever team manages it: the diagram is of the config, not of the active team. [Filtering](#filtering) says what else can be left out or put in.
* **A project's release lines are drawn inside it**, in a `lines` container beside its streams, so a project says which lines it has and not only what flows through it. There is a node for each row of the project's release targets ([CHAIN_MODEL](../design/CHAIN_MODEL.md)): a line, or each stage of a line that has a branch for each stage. The rows are newest line first, a line's stages in the order of the project's stages, and a node is labelled with the line, its stage when it has one, its branch (`no branch yet` for a line that has none, and the default branch by its name), and the series the project's stream gives the line when one does. A project that lists no line has no container, and a boundary has none.
* **A line node has the phase of its line as a class**: `development`, `ga`, `eom` or `eol`, from the `RancherLines` document and the day the command runs, drawn as the colour of its outline. A line that is not in that document has no phase class.
* **A holding is a node between two projects**, so a release is read as flowing one way: producer, holding, holder. A release starts at the artifact inside its stream and project, and a commit or a branch at the project. The holding's label is its name and what it holds (`release`, `commit` or `branch`). There is no edge from a holder back to a producer.
* **What a pin makes is a holding too.** A Go module Artifact that pins another makes a virtual holding of it in each consumer of the pinning one ([CONFIG_UNITS](../design/CONFIG_UNITS.md#o12-how-do-two-packages-say-their-versions-go-together)), which no document says, so the diagram draws it as a holding named `<holding>+<pinned artifact>` and labelled `release, pinned`, with the flow from the pinned artifact to the holder. A consumer that holds wrangler therefore shows lasso reaching it, for the lines of the wrangler holding.
* **An edit is a node after the holdings it follows.** Each `Edit` is drawn once, whatever the number of holdings it follows, with a dashed edge from each of them. Its label is its name, its strategy when it has one, and how many steps run before and after the write. The edge ends at the edit: the holding's own edge into its holder is the only one that reaches a project.
* **Stewardship is a class on the project node.** A project with a `managedBy` team is `managed`, and its label names the team; one managed by `shared` is `shared`; one outside the organization (`external`) is `external` and drawn dashed. The project marked `finalDestination` also has the `hub` class.
* **Release lines label the edge into the holder**: the lines of the `RancherLines` document that the holding applies to, newest first as the document keeps them. A holding that applies to none has an unlabelled edge.
* **Every node is labelled apart from the others.** Several streams of one thing, such as the series of a Go module, have artifacts that publish under one name, so a stream is labelled with its name and tag pattern, an artifact with its name and the name it publishes, and a holding with its name.
* **The output is deterministic.** Projects are written by slug and holdings by name, so the same config on the same day always gives the same text, whatever order the documents were read in.
* **Names are quoted.** A slug has dots and slashes that D2 reads as nesting, so every key and label is a quoted string.
* **Only text is written.** `--json` and any other output format are refused with an error that says so, and nothing is written. A format-neutral diagram is not built; each type has its own writer.

## Filtering

A config with many projects draws a diagram too large to read, and most of what is in it, such as k3s, is not ours. Three flags choose which projects are drawn, and a fourth draws one line; all apply to every type.

* **`--external` puts the external projects in.** Without it a project with `external: true` is left out. It is a default of the diagram only: the config still holds them.
* **`--project <pattern>` chooses projects by name.** It is repeatable. A pattern is matched, whatever the case, against a project's config name, its `owner/name` slug and each of its aliases, and `*` stands for any run of characters other than `/`, so `rancher/*` is the slug of every project of that organization. A pattern that starts with `!` leaves out the projects it matches.
  * With one or more patterns that do not start with `!`, the projects drawn are those that match one of them: naming a project draws it, external or not, and `--external` is not needed for it.
  * With only `!` patterns, the projects drawn are those the other rules draw, less what they match.
  * A pattern that matches no project is an error naming it, and so is a filter that leaves nothing to draw. Nothing is written.
* **`--managed-by <team>` keeps the projects a team manages.** It is not `--team`, which is the active team context of every command ([CLI_CONFIG](CLI_CONFIG.md)) and which the diagram does not follow. It is repeatable, and a project is kept when its `managedBy` is any of the names, whatever the case; `--managed-by shared` keeps the shared projects. It narrows what the other flags choose and never adds to it, so with `--project` a project must match both. An external project has no `managedBy`, so no team keeps one. A name that no project is managed by is an error naming it, as is a filter that leaves nothing; nothing is written.
* **`--line <line>` draws one release line.** The line is written with or without the `v`, in any case, and must be a line of the `RancherLines` document: any other is an error naming the lines the config has, and nothing is written. What is drawn is what is for that line:
  * a project's `lines` container has only the rows of that line, and a project that lists the line in no row and has no holding drawn at it is left out, being nothing to say of the line;
  * a stream is drawn only when it serves the line and an artifact only when the line is in its range;
  * a holding is drawn only when it applies to the line: its own range holds it and its holder and producer both list it. The boundaries are worked out from those holdings;
  * the holder edge has no line label, since every holding drawn is for the one line;
  * it narrows what the other flags choose, as `--managed-by` does, and never adds to it.
* **What leaves the view is still seen at its edge.** A project that is left out, and that a drawn project holds something of or makes something held by, is drawn as a **boundary** in the D2 diagram: a faded node with a dashed outline, labelled with its name and nothing inside it. A holding is drawn when its holder or its producer is drawn, flowing between them as ever, and from the boundary's own node, since its streams and artifacts are not drawn. A holding between two projects that are both left out is not drawn. An edit is drawn when a holding it follows is, with an edge from each of those only.
* **The mind map has no boundary node.** It draws a branch for each project drawn, and a holding is listed where it is drawn: under the artifact that reaches a project left out (`to <holder> via <holding>` names it still), and under the holder that holds something of one (`<holding>: <kind> from <producer>`).
* **A boundary is only the edge of the view.** It draws no streams or artifacts and none of the holdings that do not touch a drawn project, and whether a project is one is worked out from what is drawn, not given by a pattern.
* **The default draws the same on every run**, as does each combination of flags, and the output of no flags is the diagram of the non-external projects with the external ones that they touch as boundaries.

## Types

The optional argument, `gero config resolve graph [d2|mindmap]`, picks what is drawn, and shell completion offers both words. The default is `d2`, the diagram above; `mindmap` is the other. An unknown type is refused with an error naming it, and nothing is written.

### mindmap

A Mermaid mind map, which GitHub and most Markdown viewers render in place. It shows the same config as a looser picture, one branch per project, where the D2 diagram is a strict flow.

* **A mind map is a tree with no edges**, so what joins two projects is said on both of them. The root is the config and each project is a branch of it, in slug order, labelled with its name and, in brackets, who looks after it: `managed by <team>`, `shared` or `external`, and `hub` for the final destination. A project nobody manages has no brackets.
* **A project's streams hold its artifacts**, as in D2. Under an artifact is a node for each holding that takes it: `to <holder> via <holding>`, with the release lines it serves in brackets. A holding of a commit or a branch has the same node under its producer project.
* **A project's `lines` branch lists its release lines**, when it has any, a node for each row as in D2: `<line>[ <stage>]: <branch>[, <series>] (<phase>)`, with `no branch yet` for a line without one and the phase left out for a line not in the lines document. `--line` narrows it as it does the diagram.
* **A project's `holds` branch lists its holdings**, each as `<holding>: <kind> from <producer>`, with `, pinned` for what a pin makes, and the edits that follow it beneath.
* **Every node is a single line with an id of its own**, and quotes in a label are written as apostrophes, so a name never breaks the syntax.
* **The output is deterministic**, by project slug then holding name.

## Tests

* A fake workspace over a graph of inline documents covers: a node for every project, with its streams and artifacts nested inside it, the producer-to-holding-to-holder edges and no edge back, the stewardship and hub classes, the line labels, an edit drawn once with an edge from each holding it follows, and the same output on every run.
* The mind map is covered over the same fake workspace: one root, a branch for each project with its class, streams and artifacts nested, where an artifact reaches and what a project holds, a pin, quotes kept out of labels, and the same output each time.
* The lines are covered over the same fake workspace and a fixed day: a `lines` container in a project with a node for each line and for each stage, newest first, labelled with branch and series, a line with no branch yet, a project with no line having no container, a boundary having none, the phase class of each phase and none for a line the lines document lacks, and the same output for the same day; in the mind map the `lines` branch and its texts.
* `--line` is covered with them: the rows of one line only, a line written with or without the `v` and in any case, a stream that does not serve it and an artifact out of its range not drawn, a holding that does not apply not drawn and the boundaries following from those that are, a chosen project with nothing for the line left out, no line label on the holder edge, the mind map narrowed the same, and an unknown line refused with the lines the config has.
* An unknown type is refused and the argument defaults to D2, and completion offers the types.
* The filters are covered over the same fake workspace: an external project is a boundary and not a full node until `--external`, a `--project` pattern by name, by slug, by alias and by glob, a `!` pattern, a named external project drawn without `--external`, a holding to a project left out kept and one between two left out dropped, an edit following only holdings that are dropped not drawn, a pattern that matches nothing refused, `--managed-by` by name, by several names and by `shared`, with `--project` as both at once, a team nobody manages refused, and the mind map keeping its holdings without a boundary node.
* An error from the workspace is returned as it is.
