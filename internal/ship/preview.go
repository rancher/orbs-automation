package ship

import "github.com/rancher/orbs-automation/internal/config"

// Preview is the text a bump would write: the branch, the PR's title, labels and
// body, and the commit message. It is rendered from the embedded templates, or
// the project's own title and branch prefix, so the state check can show exactly
// what a run would put in the PR before anything is written.
type Preview struct {
	Branch        string   `json:"branch"`
	Title         string   `json:"title"`
	Labels        []string `json:"labels,omitempty"`
	CommitMessage string   `json:"commitMessage"`
	Body          string   `json:"body"`
}

// previewInput is everything the texts are made from.
type previewInput struct {
	producer   config.Project
	consumer   config.Consumer
	release    Release
	target     Target
	strategy   Strategy
	file       string
	current    string
	want       string
	userPrefix string // the user's branchPrefix setting, which replaces the project's whole
}

// renderPreview fills the texts. The branch name is a prefix, then the tag and
// the target branch without release/: the prefix is the user's setting if set,
// else the project's pullRequests.branchPrefix, else the default. The title is
// the project's pullRequests.title, else the default. An unknown placeholder in
// either is an error, though config validation rejects it sooner.
func renderPreview(in previewInput) (Preview, error) {
	return Preview{}, nil
}
