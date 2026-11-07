package ship

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	domain "github.com/rancher/orbs-automation/internal/ship"
	"github.com/rancher/orbs-automation/internal/ui/style"
)

// ErrNeedsApproval is returned when the run is going to push and open PRs, and
// there is no terminal to ask on and no --yes.
var ErrNeedsApproval = errors.New("this pushes branches and opens PRs and needs approval, but there is no terminal to ask on: pass --yes")

// question is the one line asked before anything is written: what is pushed
// where, and which PRs are opened and closed.
func question(c domain.Confirmation) string {
	var parts []string
	if len(c.Opens) > 0 {
		noun := "branch"
		prs := "PR"
		if len(c.Opens) > 1 {
			noun, prs = "branches", "PRs"
		}
		replacing := ""
		if c.MayReplace {
			replacing = fmt.Sprintf(", replacing the branch on %s if an earlier attempt left one of ours", c.Push.Remote)
		}
		parts = append(parts, fmt.Sprintf("Push the %s for %s to %s (%s)%s and open %d %s in %s",
			noun, strings.Join(c.Opens, ", "), c.Push.Remote, c.Push.Role, replacing, len(c.Opens), prs, c.Consumer))
	}
	if len(c.Refreshes) > 0 {
		verb := "rebuild"
		if len(parts) == 0 {
			verb = "Rebuild"
		}
		noun := "branch"
		if len(c.Refreshes) > 1 {
			noun = "branches"
		}
		parts = append(parts, fmt.Sprintf("%s the %s for %s and push over %s to %s (%s) if %s differ",
			verb, noun, strings.Join(c.Refreshes, ", "), "what is there", c.Push.Remote, c.Push.Role, differ(len(c.Refreshes))))
	}
	if len(c.Closes) > 0 {
		verb := "close"
		if len(parts) == 0 {
			verb = "Close"
		}
		in := ""
		if len(parts) == 0 {
			in = " in " + c.Consumer
		}
		parts = append(parts, fmt.Sprintf("%s %s%s", verb, refNumbers(c.Closes), in))
	}
	return strings.Join(parts, ", then ") + "? [y/N] "
}

// differ is "it differs" or "they differ".
func differ(n int) string {
	if n > 1 {
		return "they"
	}
	return "it"
}

// confirmer returns the function that asks before a run writes. With yes it says
// yes without asking. Without a terminal it is an error, so a forgotten flag in
// CI is a loud failure and not a hang or a silent yes. Otherwise it writes the
// question to out and reads the answer from in, and only y or yes is yes.
func confirmer(in io.Reader, out io.Writer, interactive, yes bool, st *style.Renderer) func(context.Context, domain.Confirmation) (bool, error) {
	_ = st
	return func(_ context.Context, c domain.Confirmation) (bool, error) {
		if yes {
			return true, nil
		}
		if !interactive {
			return false, ErrNeedsApproval
		}
		if _, err := fmt.Fprint(out, "\n"+question(c)); err != nil {
			return false, err
		}
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, nil
		}
		return false, nil
	}
}
