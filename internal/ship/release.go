package ship

import (
	"errors"

	"github.com/rancher/orbs-automation/internal/tagscheme"
)

// ErrLineMismatch is returned when a release is asked to go to a line that is
// not the one it is in, for a producer whose releases follow the lines.
var ErrLineMismatch = errors.New("release is in another line")

// Release is the release being shipped, as the tag scheme of its stream resolves it.
type Release = tagscheme.Release
