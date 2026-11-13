package clicfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkstationSigns(t *testing.T) {
	present := func(string) bool { return true }
	absent := func(string) bool { return false }
	runner := map[string]string{"GERO_CONTEXT": "ci", "GITHUB_ACTIONS": "true", "GITHUB_RUN_ID": "1"}
	tests := []struct {
		name   string
		env    map[string]string
		tty    bool
		exists func(string) bool
		want   []string
	}{
		{"not forced: nothing to report", map[string]string{"CI": "true"}, true, present, nil},
		{"forced on a real runner", runner, false, absent, nil},
		{"forced with a TTY on a runner", runner, true, absent, []string{"stdin and stdout are a terminal"}},
		{"forced with no runner variables", map[string]string{"GERO_CONTEXT": "ci"}, false, absent, []string{"no GitHub Actions runner variables are set"}},
		{"any one runner variable counts", map[string]string{"GERO_CONTEXT": "ci", "RUNNER_OS": "Linux"}, false, absent, nil},
		{"user file at the default path", map[string]string{"GERO_CONTEXT": "ci", "GITHUB_ACTIONS": "true"}, false, present,
			[]string{"a user file exists at /home/dev/.config/gero/config.yaml"}},
		{"GERO_CONFIG names the file on purpose", map[string]string{"GERO_CONTEXT": "ci", "GITHUB_ACTIONS": "true", "GERO_CONFIG": "/x.yaml"}, false, present, nil},
		{"all three", map[string]string{"GERO_CONTEXT": "ci"}, true, present, []string{
			"no GitHub Actions runner variables are set",
			"stdin and stdout are a terminal",
			"a user file exists at /home/dev/.config/gero/config.yaml",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, WorkstationSigns(env(tt.env), "/home/dev", tt.tty, tt.exists))
		})
	}
}
