package ship

import (
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/ship/templates"
)

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
	prs := in.consumer.Project.PullRequests
	tag := in.release.Tag
	branchLabel := strings.TrimPrefix(in.target.Branch, "release/")
	marker := Marker{
		Source: in.producer.Repo.Name, Consumer: in.consumer.Project.Repo.Name,
		Target: in.target.Branch, Version: tag,
	}
	vars := map[string]string{
		"project":          in.producer.Name,
		"source":           in.producer.Name,
		"sourceRepo":       in.producer.Repo.Name,
		"tag":              tag,
		"version":          strings.TrimPrefix(tag, "v"),
		"branch":           branchLabel,
		"targetBranch":     in.target.Branch,
		"consumer":         in.consumer.Project.Repo.Name,
		"sourceReleaseURL": "https://github.com/" + in.producer.Repo.Name + "/releases/tag/" + tag,
		"file":             in.file,
		"from":             in.current,
		"to":               in.want,
		"changes":          changes(in),
		"marker":           marker.String(),
	}

	prefix := firstNonEmpty(in.userPrefix, prs.BranchPrefix, templates.DefaultBranchPrefix())
	renderedPrefix, err := templates.Render(prefix, vars)
	if err != nil {
		return Preview{}, fmt.Errorf("branch prefix %q: %w", prefix, err)
	}
	titleTemplate := firstNonEmpty(prs.Title, templates.DefaultTitle())
	title, err := templates.Render(titleTemplate, vars)
	if err != nil {
		return Preview{}, fmt.Errorf("title %q: %w", titleTemplate, err)
	}
	commit, err := templates.Render(templates.CommitMessage(), vars)
	if err != nil {
		return Preview{}, fmt.Errorf("commit message: %w", err)
	}
	body, err := templates.Render(templates.Body(), vars)
	if err != nil {
		return Preview{}, fmt.Errorf("body: %w", err)
	}
	return Preview{
		Branch: renderedPrefix + tag + "-" + branchLabel, Title: title, Labels: prs.Labels,
		CommitMessage: commit, Body: body,
	}, nil
}

// changes lists what a bump changes, from what the strategy did and the
// post-edit commands that run, one bullet each.
func changes(in previewInput) string {
	lines := []string{"- " + in.strategy.Describe(Params(in.consumer.Source.Update.Params), in.current, in.want)}
	for _, cmd := range in.consumer.Source.Update.PostEdit {
		lines = append(lines, "- Ran `"+cmd+"`")
	}
	return strings.Join(lines, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
