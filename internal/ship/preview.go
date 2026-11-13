package ship

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/rancher/orbs-automation/internal/bump"
	"github.com/rancher/orbs-automation/internal/chain"
	"github.com/rancher/orbs-automation/internal/gero/texts"
	"github.com/rancher/orbs-automation/internal/platform/templates"
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
	// Steps are the steps that run around the write, in the order they run, each
	// with the commit it makes when it changes files. Whether it does is only known
	// once it has run, so these are the commits that may be made besides the write's.
	Steps []StepPreview `json:"steps,omitempty"`
}

// StepPreview is a step of the bump and the commit it would make.
type StepPreview struct {
	Phase string `json:"phase"` // where it runs: preEdit or postEdit
	Edit  string `json:"edit"`  // the edit it is a step of
	Run   string `json:"run"`   // the command
	// BeforeEdit says the step runs before the write, so its commit comes first.
	BeforeEdit    bool   `json:"beforeEdit,omitempty"`
	CommitMessage string `json:"commitMessage"`
}

// previewInput is everything the texts are made from.
type previewInput struct {
	producer   *chain.Project
	consumer   *chain.Project
	release    Release
	target     Target
	checks     []bump.HoldingCheck // the holdings that change: the file, the old and new values and the change of each
	steps      []bump.StepRef      // the steps that run around the write for this branch, in order, with their phase
	skipped    []bump.StepRef      // the steps the producer's team does not let gero run, which the body lists
	pins       []Companion         // the releases of pinned modules this bump takes with it, which the body says why
	env        map[string]string   // what the steps run in, which the body says
	userPrefix string              // the user's branchPrefix setting, which replaces the project's whole
}

// renderPreview fills the texts. The branch name is a prefix, then the tag and
// the target branch without release/: the prefix is the user's setting if set,
// else the project's pullRequests.branchPrefix, else the default. The title is
// the project's pullRequests.title, else the default. An unknown placeholder in
// either is an error, though config validation rejects it sooner.
func renderPreview(in previewInput) (Preview, error) {
	prs := in.consumer.PullRequests()
	tag := in.release.Tag
	branchLabel := strings.TrimPrefix(in.target.Branch, "release/")
	marker := Marker{
		Source: in.producer.Slug(), Consumer: in.consumer.Slug(),
		Target: in.target.Branch, Version: tag,
	}
	vars := textVars(in)
	vars["companions"] = companionsText(in.pins)
	vars["changes"] = changes(in)
	vars["marker"] = marker.String()

	prefix := firstNonEmpty(in.userPrefix, prs.BranchPrefix, texts.DefaultBranchPrefix())
	renderedPrefix, err := templates.Render(prefix, vars)
	if err != nil {
		return Preview{}, fmt.Errorf("branch prefix %q: %w", prefix, err)
	}
	defaultTitle, defaultCommit := texts.DefaultTitle(), texts.CommitMessage()
	if len(in.pins) > 0 { // lasso goes with wrangler: the title and the commit say so
		defaultTitle, defaultCommit = texts.DefaultTitleWith(), texts.CommitMessageWith()
	}
	titleTemplate := firstNonEmpty(prs.Title, defaultTitle)
	title, err := templates.Render(titleTemplate, vars)
	if err != nil {
		return Preview{}, fmt.Errorf("title %q: %w", titleTemplate, err)
	}
	commit, err := templates.Render(defaultCommit, vars)
	if err != nil {
		return Preview{}, fmt.Errorf("commit message: %w", err)
	}
	body, err := templates.Render(texts.Body(), vars)
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
		message, err := stepMessage(in)(bump.Unit{Kind: bump.UnitStep, Phase: ref.Phase, Edit: ref.Edit, Index: ref.Index, Run: ref.Step.Run})
		if err != nil {
			return nil, fmt.Errorf("commit message for %s: %w", strings.Join(ref.Step.Run, " "), err)
		}
		out = append(out, StepPreview{
			Phase: string(ref.Phase), Edit: ref.Edit, Run: strings.Join(ref.Step.Run, " "),
			BeforeEdit: ref.Phase == bump.PreEdit, CommitMessage: message,
		})
	}
	return out, nil
}

// companionsText says what goes with the release as a title does: "lasso to
// v0.2.10", and for more than one "a to v1, b to v2 and c to v3".
func companionsText(pins []Companion) string {
	parts := make([]string, len(pins))
	for i, c := range pins {
		parts[i] = c.Name + " to " + c.Tag
	}
	switch len(parts) {
	case 0, 1:
		return strings.Join(parts, "")
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// textVars are the variables of the texts that only need to know what the bump
// is: the producer, the consumer, the release and the target.
func textVars(in previewInput) map[string]string {
	return map[string]string{
		"project":          in.producer.Name(),
		"source":           in.producer.Name(),
		"sourceRepo":       in.producer.Slug(),
		"tag":              in.release.Tag,
		"version":          in.producer.ReleaseVersion(in.release.Tag),
		"branch":           strings.TrimPrefix(in.target.Branch, "release/"),
		"targetBranch":     in.target.Branch,
		"consumer":         in.consumer.Slug(),
		"key":              BranchKey(in.producer.Name(), in.release.Tag, in.target.Branch),
		"sourceReleaseURL": "https://github.com/" + in.producer.Slug() + "/releases/tag/" + in.release.Tag,
		"file":             changedFiles(in.checks),
		"from":             joined(in.checks, func(c bump.HoldingCheck) string { return c.Current }),
		"to":               joined(in.checks, func(c bump.HoldingCheck) string { return c.Want }),
	}
}

// stepMessage returns the function that makes the commit message of a step that
// changed files: what ran and where it fits in the bump, from the embedded
// template. The preview (stepPreviews) and a run (Apply) both use it, so what is
// previewed is what is committed; whether a step changes files, and so commits at
// all, is only known once it has run.
func stepMessage(in previewInput) func(bump.Unit) (string, error) {
	return func(u bump.Unit) (string, error) {
		vars := textVars(in)
		vars["command"] = strings.Join(u.Run, " ")
		vars["phase"] = string(u.Phase)
		return templates.Render(texts.StepCommitMessage(), vars)
	}
}

// blankLines is two or more blank lines in a row.
var blankLines = regexp.MustCompile(`\n{3,}`)

// bullets lists changes, one bullet each.
func bullets(changes []bump.Change) string {
	lines := make([]string, len(changes))
	for i, c := range changes {
		lines[i] = "- " + c.Description
	}
	return strings.Join(lines, "\n")
}

// strategyMessage returns the function that makes the commit message of a
// strategy that changed files: which strategy ran and what it changed, from the
// embedded template.
func strategyMessage(in previewInput) func(bump.Unit) (string, error) {
	return func(u bump.Unit) (string, error) {
		vars := textVars(in)
		vars["strategy"] = u.Strategy
		vars["edit"] = u.Edit
		vars["changes"] = bullets(u.Changes)
		msg, err := templates.Render(texts.StrategyCommitMessage(), vars)
		if err != nil {
			return "", err
		}
		// A strategy that names no change leaves two blank lines where the list was.
		return blankLines.ReplaceAllString(msg, "\n\n"), nil
	}
}

// joined is a value of each check, in order, with the repeats left out and the
// rest joined by a comma: one value when the holdings share it.
func joined(checks []bump.HoldingCheck, of func(bump.HoldingCheck) string) string {
	var seen []string
	for _, c := range checks {
		if v := of(c); !slices.Contains(seen, v) {
			seen = append(seen, v)
		}
	}
	return strings.Join(seen, ", ")
}

// changedFiles is the files that a bump changes, in order and without repeats,
// joined by a comma: every location of a holding that is not up to date.
func changedFiles(checks []bump.HoldingCheck) string {
	var files []string
	for _, c := range checks {
		for _, change := range c.Changes() {
			if !slices.Contains(files, change.Path) {
				files = append(files, change.Path)
			}
		}
	}
	return strings.Join(files, ", ")
}

// changes lists what a bump changes, from what each holding changes and the
// commands that run around the write, one bullet each.
func changes(in previewInput) string {
	var lines []string
	for _, c := range in.checks {
		for _, change := range c.Changes() {
			lines = append(lines, "- "+change.Description)
		}
	}
	for _, ref := range in.steps {
		lines = append(lines, "- Ran `"+strings.Join(ref.Step.Run, " ")+"`")
	}
	if len(in.env) > 0 {
		pairs := make([]string, 0, len(in.env))
		for _, name := range slices.Sorted(maps.Keys(in.env)) {
			pairs = append(pairs, "`"+name+"="+in.env[name]+"`")
		}
		lines = append(lines, "- The steps ran with "+strings.Join(pairs, ", "))
	}
	for _, c := range in.pins {
		lines = append(lines, "- `"+c.Module+"` goes to "+c.Tag+", which "+c.RequiredBy+" requires")
	}
	for _, ref := range in.skipped {
		lines = append(lines, "- Skipped `"+strings.Join(ref.Step.Run, " ")+"`: it runs from "+in.producer.Slug()+", "+managedByWords(in.producer)+", so this PR may be incomplete")
	}
	return strings.Join(lines, "\n")
}

// managedByWords says who manages a project, for the PR to read: managed by a
// team, or by none that is named.
func managedByWords(p *chain.Project) string {
	if team := p.ManagedBy(); team != "" {
		return "which " + team + " manages"
	}
	return "which no team names"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
