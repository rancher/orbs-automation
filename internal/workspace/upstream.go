package workspace

import "strings"

// UpstreamFor returns the name of the remote that is the real source of the
// repository slug: its repos.overrides entry when the user file has one, else
// the global upstream setting. It is empty when neither is set. The raw
// settings stay in clicfg.Settings; anything that acts on a remote asks here.
func (w *Workspace) UpstreamFor(slug string) string {
	for key, o := range w.settings.Repos.Overrides {
		if strings.EqualFold(key, slug) && o.Upstream != "" {
			return o.Upstream
		}
	}
	return w.settings.Upstream
}
