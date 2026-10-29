package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// recordTransport answers every request with an empty user and keeps the last
// request it saw.
func recordTransport(last **http.Request) http.RoundTripper {
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*last = r
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		_, _ = rec.WriteString(`{"login":"octo"}`)
		return rec.Result(), nil
	})
}

func TestNewClientTargetsTheHostsAPI(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantURL string
	}{
		{"github.com uses the public API", "github.com", "https://api.github.com/user"},
		{"enterprise server uses /api/v3", "ghe.example.com", "https://ghe.example.com/api/v3/user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var last *http.Request
			c, err := NewClient(Credentials{Host: tt.host, Token: "tok"},
				gogithub.WithTransport(recordTransport(&last)))
			require.NoError(t, err)

			user, _, err := c.Users.Get(context.Background(), "")

			require.NoError(t, err)
			assert.Equal(t, "octo", user.GetLogin())
			assert.Equal(t, tt.wantURL, last.URL.String())
			assert.Equal(t, "Bearer tok", last.Header.Get("Authorization"))
		})
	}
}

func TestNewClientRejectsAnEmptyToken(t *testing.T) {
	c, err := NewClient(Credentials{Host: "github.com"})

	require.ErrorIs(t, err, ErrNoToken)
	assert.Nil(t, c)
}

func TestNewClientRejectsHostsItCannotAddressYet(t *testing.T) {
	for _, host := range []string{"acme.ghe.com", "github.localhost"} {
		t.Run(host, func(t *testing.T) {
			c, err := NewClient(Credentials{Host: host, Token: "tok"})

			require.ErrorIs(t, err, ErrUnsupportedHost)
			assert.ErrorContains(t, err, host)
			assert.Nil(t, c)
		})
	}
}
