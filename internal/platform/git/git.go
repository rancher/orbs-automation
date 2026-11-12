// Package git holds the pure logic about git refs, tags and commits. It does no
// I/O, never reads the environment, and imports neither go-git nor os/exec.
// Repository access lives in internal/platform/git/repo.
package git

const shortLength = 7

// Short returns the first 7 characters of sha, or all of it when shorter. It
// is how a CI commit is abbreviated.
func Short(sha string) string {
	return sha[:min(len(sha), shortLength)]
}
