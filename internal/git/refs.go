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

// Branch returns the branch of the name and whether there is one.
//
// STUB: reports no branch until a later commit.
func (r Refs) Branch(name string) (Branch, bool) { return Branch{}, false }
