package github

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForkExistsWhenTheUsersRepositoryIsAForkOfIt(t *testing.T) {
	var last *http.Request

	ok, err := ForkExists(t.Context(), Credentials{Host: "github.com", Token: "tok"}, "danpock", "rancher/rancher",
		respondWith(&last, http.StatusOK, `{"full_name":"danpock/rancher","fork":true,"parent":{"full_name":"rancher/rancher"}}`))

	require.NoError(t, err)
	assert.True(t, ok)
	require.NotNil(t, last, "the stub must have made a request")
	assert.Equal(t, "/repos/danpock/rancher", last.URL.Path)
	assert.Equal(t, "Bearer tok", last.Header.Get("Authorization"))
}

func TestForkExistsComparesTheParentWithoutRegardToCase(t *testing.T) {
	var last *http.Request

	ok, err := ForkExists(t.Context(), Credentials{Host: "github.com", Token: "tok"}, "danpock", "rancher/rancher",
		respondWith(&last, http.StatusOK, `{"fork":true,"parent":{"full_name":"Rancher/Rancher"}}`))

	require.NoError(t, err)
	assert.True(t, ok)
}

func TestForkExistsIsFalseForNoRepositoryAndNoError(t *testing.T) {
	var last *http.Request

	ok, err := ForkExists(t.Context(), Credentials{Host: "github.com", Token: "tok"}, "danpock", "rancher/rancher",
		respondWith(&last, http.StatusNotFound, `{"message":"Not Found"}`))

	require.NoError(t, err)
	assert.False(t, ok)
	assert.NotNil(t, last, "the stub must have made a request")
}

func TestForkExistsIsFalseForARepositoryThatIsNotAForkOfIt(t *testing.T) {
	for name, body := range map[string]string{
		"not a fork":      `{"fork":false}`,
		"a fork of other": `{"fork":true,"parent":{"full_name":"someone/rancher"}}`,
	} {
		var last *http.Request

		ok, err := ForkExists(t.Context(), Credentials{Host: "github.com", Token: "tok"}, "danpock", "rancher/rancher",
			respondWith(&last, http.StatusOK, body))

		require.NoError(t, err, name)
		assert.False(t, ok, name)
		assert.NotNil(t, last, name+": the stub must have made a request")
	}
}

func TestForkExistsAFailedLookupIsAnErrorThatNamesTheRepository(t *testing.T) {
	var last *http.Request

	_, err := ForkExists(t.Context(), Credentials{Host: "github.com", Token: "tok"}, "danpock", "rancher/rancher",
		respondWith(&last, http.StatusInternalServerError, `{"message":"boom"}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "danpock/rancher")
}

func TestForkExistsReadsAPublicRepositoryWithNoToken(t *testing.T) {
	var last *http.Request

	ok, err := ForkExists(t.Context(), Credentials{Host: "github.com"}, "danpock", "rancher/rancher",
		respondWith(&last, http.StatusOK, `{"fork":true,"parent":{"full_name":"rancher/rancher"}}`))

	require.NoError(t, err)
	assert.True(t, ok)
	require.NotNil(t, last, "the stub must have made a request")
	assert.Empty(t, last.Header.Get("Authorization"))
}
