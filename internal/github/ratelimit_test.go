package github

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoreRateLimit(t *testing.T) {
	var last *http.Request
	body := `{"resources":{"core":{"limit":5000,"remaining":4980,"used":20,"reset":1767225600},` +
		`"search":{"limit":30,"remaining":30,"used":0,"reset":1767225660}}}`

	got, err := CoreRateLimit(context.Background(), Credentials{Host: "github.com", Token: "tok"},
		respondWith(&last, http.StatusOK, body))

	require.NoError(t, err)
	assert.Equal(t, RateLimit{Limit: 5000, Remaining: 4980, Reset: time.Unix(1767225600, 0).UTC()}, got)
	require.NotNil(t, last, "the stub must have made a request")
	assert.Equal(t, "https://api.github.com/rate_limit", last.URL.String())
	assert.Equal(t, "Bearer tok", last.Header.Get("Authorization"))
}

func TestCoreRateLimitOnAnEnterpriseHost(t *testing.T) {
	var last *http.Request

	_, err := CoreRateLimit(context.Background(), Credentials{Host: "ghe.example.com", Token: "tok"},
		respondWith(&last, http.StatusOK, `{"resources":{"core":{"limit":15000,"remaining":1,"reset":1}}}`))

	require.NoError(t, err)
	require.NotNil(t, last)
	assert.Equal(t, "https://ghe.example.com/api/v3/rate_limit", last.URL.String())
}

func TestCoreRateLimitRejectedToken(t *testing.T) {
	var last *http.Request

	got, err := CoreRateLimit(context.Background(), Credentials{Host: "github.com", Token: "stale"},
		respondWith(&last, http.StatusUnauthorized, `{"message":"Bad credentials"}`))

	require.ErrorIs(t, err, ErrBadCredentials)
	assert.ErrorContains(t, err, "github.com")
	assert.Equal(t, RateLimit{}, got)
}

func TestCoreRateLimitOtherFailureIsNotBadCredentials(t *testing.T) {
	var last *http.Request

	_, err := CoreRateLimit(context.Background(), Credentials{Host: "github.com", Token: "tok"},
		respondWith(&last, http.StatusInternalServerError, `{"message":"boom"}`))

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrBadCredentials)
	assert.ErrorContains(t, err, "boom")
}

func TestCoreRateLimitWithoutACoreEntry(t *testing.T) {
	var last *http.Request

	_, err := CoreRateLimit(context.Background(), Credentials{Host: "github.com", Token: "tok"},
		respondWith(&last, http.StatusOK, `{"resources":{"search":{"limit":30,"remaining":30,"reset":1}}}`))

	require.Error(t, err)
	assert.ErrorContains(t, err, "core")
}

func TestCoreRateLimitWithoutATokenOrWithAnUnsupportedHost(t *testing.T) {
	_, err := CoreRateLimit(context.Background(), Credentials{Host: "github.com"})
	assert.ErrorIs(t, err, ErrNoToken)

	_, err = CoreRateLimit(context.Background(), Credentials{Host: "acme.ghe.com", Token: "tok"})
	assert.ErrorIs(t, err, ErrUnsupportedHost)
}
