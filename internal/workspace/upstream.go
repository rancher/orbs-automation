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

// PushSettings are the user's settings that decide where a branch for a
// repository is pushed, as the user file has them. The layers are put together
// by the command that pushes, and the project's own pullRequests.pushTo is its
// config, so it is not here.
type PushSettings struct {
	UserRepo    string // repos.overrides.<slug>.pushTo, "" when not set
	UserDefault string // pushTo, "" when not set
	Fork        string // the remote that holds the engineer's fork, "" when not set
	Upstream    string // the remote that is the real repository (UpstreamFor)
	GitHubUser  string // the owner of the fork, for a PR's head
}

// PushSettings returns the settings that decide where a branch for slug is pushed.
//
// STUB: empty until a later commit.
func (w *Workspace) PushSettings(slug string) PushSettings { return PushSettings{} }
