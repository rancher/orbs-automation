package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAuth answers from fixed maps and records the hosts it was asked about.
type fakeAuth struct {
	tokens         map[string][2]string // host -> token, source
	defaultHost    [2]string            // host, source
	enterprise     map[string]bool
	askedTokenFor  []string
	askedDefaultOf int
}

func (f *fakeAuth) TokenForHost(host string) (string, string) {
	f.askedTokenFor = append(f.askedTokenFor, host)
	t := f.tokens[host]
	return t[0], t[1]
}

func (f *fakeAuth) DefaultHost() (string, string) {
	f.askedDefaultOf++
	return f.defaultHost[0], f.defaultHost[1]
}

func (f *fakeAuth) IsEnterprise(host string) bool { return f.enterprise[host] }

func TestResolve(t *testing.T) {
	tests := []struct {
		name string
		auth *fakeAuth
		host string
		want Credentials
	}{
		{
			name: "default host and its token",
			auth: &fakeAuth{
				defaultHost: [2]string{"github.com", "default"},
				tokens:      map[string][2]string{"github.com": {"t1", "GH_TOKEN"}},
			},
			want: Credentials{Host: "github.com", HostSource: "default", Token: "t1", TokenSource: "GH_TOKEN"},
		},
		{
			name: "default host named by GH_HOST",
			auth: &fakeAuth{
				defaultHost: [2]string{"ghe.example.com", "GH_HOST"},
				tokens:      map[string][2]string{"ghe.example.com": {"t2", "oauth_token"}},
			},
			want: Credentials{Host: "ghe.example.com", HostSource: "GH_HOST", Token: "t2", TokenSource: "oauth_token"},
		},
		{
			name: "explicit host skips the default",
			auth: &fakeAuth{
				defaultHost: [2]string{"github.com", "default"},
				tokens:      map[string][2]string{"ghe.example.com": {"t3", "gh"}},
			},
			host: "ghe.example.com",
			want: Credentials{Host: "ghe.example.com", HostSource: SourceExplicit, Token: "t3", TokenSource: "gh"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.auth, tt.host)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, []string{tt.want.Host}, tt.auth.askedTokenFor)
			if tt.host != "" {
				assert.Zero(t, tt.auth.askedDefaultOf, "an explicit host must not consult the default")
			}
		})
	}
}

func TestResolveNoToken(t *testing.T) {
	tests := []struct {
		name string
		auth *fakeAuth
		host string
		want string
	}{
		{
			name: "github.com names GH_TOKEN and GITHUB_TOKEN",
			auth: &fakeAuth{defaultHost: [2]string{"github.com", "default"}},
			want: `no GitHub token found for github.com: set GH_TOKEN or GITHUB_TOKEN, or run "gh auth login"`,
		},
		{
			name: "enterprise host names the enterprise variables",
			auth: &fakeAuth{enterprise: map[string]bool{"ghe.example.com": true}},
			host: "ghe.example.com",
			want: `no GitHub token found for ghe.example.com: set GH_ENTERPRISE_TOKEN or GITHUB_ENTERPRISE_TOKEN, ` +
				`or run "gh auth login --hostname ghe.example.com"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.auth, tt.host)

			require.ErrorIs(t, err, ErrNoToken)
			assert.EqualError(t, err, tt.want)
			assert.Equal(t, Credentials{}, got)
		})
	}
}

func TestResolveNoTokenErrorCarriesTheHostAndTheHint(t *testing.T) {
	tests := []struct {
		name     string
		auth     *fakeAuth
		host     string
		wantHost string
		wantHint string
	}{
		{
			name:     "default host",
			auth:     &fakeAuth{defaultHost: [2]string{"github.com", "default"}},
			wantHost: "github.com",
			wantHint: `set GH_TOKEN or GITHUB_TOKEN, or run "gh auth login"`,
		},
		{
			name:     "enterprise host",
			auth:     &fakeAuth{enterprise: map[string]bool{"ghe.example.com": true}},
			host:     "ghe.example.com",
			wantHost: "ghe.example.com",
			wantHint: `set GH_ENTERPRISE_TOKEN or GITHUB_ENTERPRISE_TOKEN, or run "gh auth login --hostname ghe.example.com"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(tt.auth, tt.host)

			var noToken *NoTokenError
			require.ErrorAs(t, err, &noToken)
			assert.Equal(t, tt.wantHost, noToken.Host)
			assert.Equal(t, tt.wantHint, noToken.Hint)
			assert.ErrorIs(t, err, ErrNoToken)
		})
	}
}
