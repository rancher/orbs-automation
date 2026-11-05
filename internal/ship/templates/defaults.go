package templates

// The texts gero embeds, used when a project's config does not set its own. They
// are generic: nothing in them names a project, and what varies comes from the
// placeholders (TextVars). They are plain files under defaults/ so they read as
// the text they produce.

// DefaultTitle is the PR title when pullRequests.title is not set.
func DefaultTitle() string { return "" }

// DefaultBranchPrefix is the start of the branch name when neither the user nor
// the project sets one. The rest of the name is the tag and the target branch.
func DefaultBranchPrefix() string { return "" }

// CommitMessage is the commit message of a bump: a subject, a short account of
// where it came from, and the trailers.
func CommitMessage() string { return "" }

// Body is the PR body of a bump.
func Body() string { return "" }
