package git

// Branch is a branch name and the commit at its head.
type Branch struct {
	Name   string
	Commit string
}

// Refs is the tags and branches of a repository, each sorted by name.
type Refs struct {
	Tags     []Tag
	Branches []Branch
}
