package git

import (
	"path"

	"github.com/rancher/orbs-automation/internal/semver"
)

// Tag is a tag name and the commit it points at, with an annotated tag already
// resolved to its commit.
type Tag struct {
	Name   string
	Commit string
}

// Latest is the highest stable tag and the highest prerelease tag that is newer
// than it. Each is nil when there is none.
type Latest struct {
	Stable     *Tag
	Prerelease *Tag
}

// LatestReleases finds the highest stable and the highest prerelease tag among
// tags whose name matches the glob pattern (release.tagPattern, such as "v*") and read
// as a version. The highest is by version precedence, not by date, so a patch
// tag on an old line does not look newest. Tags that are not versions, such as
// the rolling main-<sha> tags, are ignored. A prerelease is reported only when
// it is newer than the latest stable tag, since an older one is history (v2.15.3
// supersedes v2.15.3-rc2). A pattern that is not valid matches nothing.
func LatestReleases(tags []Tag, pattern string) Latest {
	type candidate struct {
		tag Tag
		v   semver.Version
	}
	var stable, pre *candidate
	for _, tag := range tags {
		if ok, err := path.Match(pattern, tag.Name); err != nil || !ok {
			continue
		}
		v, ok := semver.Parse(tag.Name)
		if !ok {
			continue
		}
		best := &stable
		if v.IsPrerelease() {
			best = &pre
		}
		if *best == nil || semver.Compare(v, (*best).v) > 0 {
			*best = &candidate{tag: tag, v: v}
		}
	}
	var out Latest
	if stable != nil {
		out.Stable = &stable.tag
	}
	if pre != nil && (stable == nil || semver.Compare(pre.v, stable.v) > 0) {
		out.Prerelease = &pre.tag
	}
	return out
}
