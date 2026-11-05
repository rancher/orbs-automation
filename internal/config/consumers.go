package config

// Consumer is a Project that takes updates from another, with the source entry
// that says how.
type Consumer struct {
	Project Project
	Source  Source
}

// Consumers returns the projects that take updates from the project named
// producer (the document name, which is what a source's from names), each with
// the source entry that names it, in config order. Producers do not list their
// consumers, so this is the reverse of what the config says.
func (c Config) Consumers(producer string) []Consumer {
	var out []Consumer
	for _, p := range c.Projects {
		for _, s := range p.Sources {
			if s.From == producer {
				out = append(out, Consumer{Project: p, Source: s})
			}
		}
	}
	return out
}
