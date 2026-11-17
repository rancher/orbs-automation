package proposal

// Preview is the text a bump would write: the branch, the PR's title, labels and
// body, and the commit message. It is rendered from the embedded templates, or
// the project's own title and branch prefix, so the state check can show exactly
// what a run would put in the PR before anything is written.
type Preview struct {
	Branch string `json:"branch"`
	// Key is the end of the branch that gero builds (BranchKey), the same for
	// every actor, which is how another PR of the bump is found.
	Key           string   `json:"key"`
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
