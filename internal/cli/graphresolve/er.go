package graphresolve

import (
	"fmt"
	"strings"
)

// erDiagram draws the schema of the config as a Mermaid entity-relationship
// diagram (docs/specs/CONFIG_GRAPH.md, "er"). It is the same text every time and
// reads no config, so a filter, which chooses data, has nothing to choose and is
// refused.
func erDiagram(opts Options) (string, error) {
	if flag := erFilter(opts); flag != "" {
		return "", fmt.Errorf("%s chooses what data is drawn, and the er type draws the schema, which has none", flag)
	}
	return er, nil
}

// erFilter is the flag of the first filter the options set, or empty.
func erFilter(opts Options) string {
	switch {
	case opts.External:
		return "--external"
	case len(opts.Projects) > 0:
		return "--project"
	case len(opts.ManagedBy) > 0:
		return "--managed-by"
	case opts.Line != "":
		return "--line"
	}
	return ""
}

// er is the schema. An entity lists only the columns that say how it relates to
// another, not every field of its document.
var er = strings.TrimLeft(`
erDiagram
    %% Each entity is a table; the other graph types draw the rows joined across them.
    %% Holding is the junction: it joins a producer to a holder, a Project to a Project.
    Project ||--o{ Stream : releases
    Stream ||--|{ Artifact : produces
    Project ||--o{ LineBranch : lists
    Line ||--o{ LineBranch : "is branched as"
    Holding }o--|| Project : holder
    Holding }o--|| Project : producer
    Holding }o--o| Artifact : takes
    Holding }o--o| Stream : "holds the series of"
    Holding }o--o{ Line : "applies to"
    Edit }o--|{ Holding : follows
    Artifact }o--o{ Artifact : pins

    %% what a pin makes is a virtual holding: derived from the pins, never a row of its own
    %% a Line is a row of the RancherLines document, with its phase read from the day

    Project {
        string name
        string managedBy
        bool finalDestination
    }
    Stream {
        string name
        string scheme
        string tagPattern
    }
    Artifact {
        string name
        string kind
        string ref
    }
    Line {
        string id
        string phase
    }
    LineBranch {
        string line
        string branch
    }
    Holding {
        string name
        string kind
        string holder FK
        string producer FK
    }
    Edit {
        string name
        string strategy
    }
`, "\n")
