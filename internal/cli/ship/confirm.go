package ship

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	domain "github.com/rancher/orbs-automation/internal/ship"
)

// ErrNeedsApproval is returned when the run is going to push and open PRs, and
// there is no terminal to ask on and no --yes.
var ErrNeedsApproval = errors.New("this pushes branches and opens PRs and needs approval, but there is no terminal to ask on: pass --yes")

// question is the one line asked before anything is written: what is pushed
// where, and which PRs are opened and closed.
//
// STUB: empty until a later commit.
func question(c domain.Confirmation) string { return "" }

// confirmer returns the function that asks before a run writes. With yes it says
// yes without asking. Without a terminal it is an error, so a forgotten flag in
// CI is a loud failure and not a hang or a silent yes. Otherwise it writes the
// question to out and reads the answer from in, and only y or yes is yes.
//
// STUB: always no until a later commit.
func confirmer(in io.Reader, out io.Writer, interactive, yes bool) func(context.Context, domain.Confirmation) (bool, error) {
	return func(context.Context, domain.Confirmation) (bool, error) { return false, nil }
}

var _ = bufio.NewReader
var _ = strings.TrimSpace
var _ = fmt.Sprintf
