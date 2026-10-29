// Package configshow holds the logic of gero config show: it renders the
// resolved CLI Settings and the layer each value came from. It is
// framework-free: cmd/ owns the cobra wiring.
package configshow

import "github.com/rancher/orbs-automation/internal/clicfg"

// Entry is one resolved setting.
type Entry struct {
	Key    string // user-file key, dotted for nested keys (repos.paths.<slug>)
	Value  string
	Source clicfg.Source
}

// Entries flattens s into one Entry per setting, in display order.
func Entries(clicfg.Settings) []Entry {
	return nil
}

// Render formats s as clicfg.OutputText, OutputJSON or OutputEnv.
func Render(clicfg.Settings, string) (string, error) {
	return "", nil
}
