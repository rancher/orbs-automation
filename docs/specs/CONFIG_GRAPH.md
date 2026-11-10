# CONFIG_GRAPH

**Status:** implemented. `gero config resolve graph`: the whole application config drawn as a D2 diagram.

The config joins into one graph ([CHAIN_MODEL](../design/CHAIN_MODEL.md)), which the other commands read one question at a time. This command draws all of it at once, so the projects and the way releases flow between them are seen together. It reads the central config and never the network or the clock, and writes D2 source to stdout ([CLI_OUTPUT](CLI_OUTPUT.md)); render it with `d2 graph.d2 graph.svg`.

* **Projects are the primary nodes**, containers of the streams they release and, inside each stream, the artifacts it produces. They are keyed by the repository's `owner/name` slug and labelled with the config name. Every project of the config is drawn, whichever team manages it: the diagram is of the config, not of the active team.
* **A holding is a node between two projects**, so a release is read as flowing one way: producer, holding, holder. A release starts at the artifact inside its stream and project, and a commit or a branch at the project. The holding's label is its name and what it holds (`release`, `commit` or `branch`). There is no edge from a holder back to a producer.
* **An edit is a node after the holdings it follows.** Each `Edit` is drawn once, whatever the number of holdings it follows, with a dashed edge from each of them. Its label is its name, its strategy when it has one, and how many steps run before and after the write. The edge ends at the edit: the holding's own edge into its holder is the only one that reaches a project.
* **Stewardship is a class on the project node.** A project with a `managedBy` team is `managed`, and its label names the team; one managed by `shared` is `shared`; one outside the organization (`external`) is `external` and drawn dashed. The project marked `finalDestination` also has the `hub` class.
* **Release lines label the edge into the holder**: the lines of the `RancherLines` document that the holding applies to, newest first as the document keeps them. A holding that applies to none has an unlabelled edge.
* **Every node is labelled apart from the others.** Several streams of one thing, such as the series of a Go module, have artifacts that publish under one name, so a stream is labelled with its name and tag pattern, an artifact with its name and the name it publishes, and a holding with its name.
* **The output is deterministic.** Projects are written by slug and holdings by name, so the same config always gives the same text, whatever order the documents were read in.
* **Names are quoted.** A slug has dots and slashes that D2 reads as nesting, so every key and label is a quoted string.
* **Only D2 is written.** `--json` and any other output format are refused with an error that says so, and nothing is written. A format-neutral diagram is not built; another format adds its own writer beside this one.

## Tests

* A fake workspace over a graph of inline documents covers: a node for every project, with its streams and artifacts nested inside it, the producer-to-holding-to-holder edges and no edge back, the stewardship and hub classes, the line labels, an edit drawn once with an edge from each holding it follows, and the same output on every run.
* An error from the workspace is returned as it is.
