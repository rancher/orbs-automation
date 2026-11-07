package ship

import (
	"fmt"
	"strings"

	"github.com/rancher/orbs-automation/internal/config"
	"github.com/rancher/orbs-automation/internal/ship/templates"
	"github.com/rancher/orbs-automation/internal/updatedispatcher"
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
	// Steps are the steps that run around the edit, in the order they run, each
	// with the commit it makes when it changes files. Whether it does is only known
	// once it has run, so these are the commits that may be made besides the edit's.
	Steps []StepPreview `json:"steps,omitempty"`
}

// StepPreview is a step of the bump and the commit it would make.
type StepPreview struct {
	Phase string `json:"phase"` // where it runs: method preEdit, edge preEdit, edge postEdit or method postEdit
	Run   string `json:"run"`   // the command
	// BeforeEdit says the step runs before the edit, so its commit comes first.
	BeforeEdit    bool   `json:"beforeEdit,omitempty"`
	CommitMessage string `json:"commitMessage"`
}

// previewInput is everything the texts are made from.
type previewInput struct {
	producer   config.Project
	consumer   config.Project
	release    Release
	target     Target
	check      updatedispatcher.Check     // what the strategy found: the file, the old and new values and the change
	steps      []updatedispatcher.StepRef // the steps that run around the edit for this branch, in order, with their phase
	userPrefix string                     // the user's branchPrefix setting, which replaces the project's whole
}

// renderPreview fills the texts. The branch name is a prefix, then the tag and
// the target branch without release/: the prefix is the user's setting if set,
// else the project's pullRequests.branchPrefix, else the default. The title is
// the project's pullRequests.title, else the default. An unknown placeholder in
// either is an error, though config validation rejects it sooner.
func renderPreview(in previewInput) (Preview, error) {
	prs := in.consumer.PullRequests
	tag := in.release.Tag
	branchLabel := strings.TrimPrefix(in.target.Branch, "release/")
	marker := Marker{
		Source: in.producer.Repo.Name, Consumer: in.consumer.Repo.Name,
		Target: in.target.Branch, Version: tag,
	}
	vars := textVars(in)
	vars["changes"] = changes(in)
	vars["marker"] = marker.String()

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
	steps, err := stepPreviews(in)
	if err != nil {
		return Preview{}, err
	}
	return Preview{
		Branch: renderedPrefix + tag + "-" + branchLabel, Title: title, Labels: prs.Labels,
		CommitMessage: commit, Body: body, Steps: steps,
	}, nil
}

// stepPreviews is the commit each step would make, if it changes files, with the
// message a run gives it (stepMessage), so the preview shows what will be written.
func stepPreviews(in previewInput) ([]StepPreview, error) {
	var out []StepPreview
	for _, ref := range in.steps {
		message, err := stepMessage(in)(updatedispatcher.Unit{Kind: updatedispatcher.UnitStep, Phase: ref.Phase, Index: ref.Index, Run: ref.Step.Run})
		if err != nil {
			return nil, fmt.Errorf("commit message for %s: %w", strings.Join(ref.Step.Run, " "), err)
		}
		out = append(out, StepPreview{
			Phase: string(ref.Phase), Run: strings.Join(ref.Step.Run, " "),
			BeforeEdit: ref.Phase == updatedispatcher.MethodPreEdit || ref.Phase == updatedispatcher.EdgePreEdit, CommitMessage: message,
		})
	}
	return out, nil
}

// textVars are the variables of the texts that only need to know what the bump
// is: the producer, the consumer, the release and the target.
func textVars(in previewInput) map[string]string {
	return map[string]string{
		"project":          in.producer.Name,
		"source":           in.producer.Name,
		"sourceRepo":       in.producer.Repo.Name,
		"tag":              in.release.Tag,
		"version":          strings.TrimPrefix(in.release.Tag, "v"),
		"branch":           strings.TrimPrefix(in.target.Branch, "release/"),
		"targetBranch":     in.target.Branch,
		"consumer":         in.consumer.Repo.Name,
		"sourceReleaseURL": "https://github.com/" + in.producer.Repo.Name + "/releases/tag/" + in.release.Tag,
		"file":             in.check.Change.Path,
		"from":             in.check.Current,
		"to":               in.check.Want,
	}
}

// stepMessage returns the function that makes the commit message of a step that
// changed files: what ran and where it fits in the bump, from the embedded
// template. The preview (stepPreviews) and a run (Apply) both use it, so what is
// previewed is what is committed; whether a step changes files, and so commits at
// all, is only known once it has run.
func stepMessage(in previewInput) func(updatedispatcher.Unit) (string, error) {
	return func(u updatedispatcher.Unit) (string, error) {
		vars := textVars(in)
		vars["command"] = strings.Join(u.Run, " ")
		vars["phase"] = string(u.Phase)
		return templates.Render(templates.StepCommitMessage(), vars)
	}
}

// changes lists what a bump changes, from what the strategy did and the
// post-edit commands that run, one bullet each.
func changes(in previewInput) string {
	lines := []string{"- " + in.check.Change.Description}
	for _, ref := range in.steps {
		lines = append(lines, "- Ran `"+strings.Join(ref.Step.Run, " ")+"`")
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
