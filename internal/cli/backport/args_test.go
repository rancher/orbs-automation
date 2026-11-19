package backport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgsReadsTheProjectTheLinesAndThePR(t *testing.T) {
	for name, tc := range map[string]struct {
		args []string
		want Args
	}{
		"lines and a number": {[]string{"v2.15", "123"}, Args{Lines: []string{"v2.15"}, PR: 123}},
		"a project, several lines and #number": {
			[]string{"bro", "v2.15,2.14", "#123"}, Args{Project: "bro", Lines: []string{"v2.15", "2.14"}, PR: 123},
		},
		"a slug as the project": {
			[]string{"rancher/rancher", "release/v2.15", "7"}, Args{Project: "rancher/rancher", Lines: []string{"release/v2.15"}, PR: 7},
		},
		"a URL carries the repo": {
			[]string{"v2.15", "https://github.com/rancher/backup-restore-operator/pull/42"},
			Args{Lines: []string{"v2.15"}, PR: 42, Repo: "rancher/backup-restore-operator"},
		},
		"spaces around a line are dropped": {[]string{"v2.15, v2.14", "5"}, Args{Lines: []string{"v2.15", "v2.14"}, PR: 5}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseArgs(tc.args)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseArgsRefusesWhatIsNotAProjectLinesAndAPR(t *testing.T) {
	for name, tc := range map[string]struct {
		args []string
		msg  string
	}{
		"nothing":                 {nil, "line"},
		"only a PR":               {[]string{"123"}, "line"},
		"the last is not a PR":    {[]string{"bro", "v2.15"}, "PR"},
		"too many":                {[]string{"bro", "v2.15", "v2.14", "123"}, "comma"},
		"an empty line":           {[]string{"v2.15,,v2.14", "123"}, "empty"},
		"zero is not a PR":        {[]string{"v2.15", "0"}, "PR"},
		"a URL of something else": {[]string{"v2.15", "https://github.com/rancher/rancher/issues/42"}, "PR"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseArgs(tc.args)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.msg)
		})
	}
}
