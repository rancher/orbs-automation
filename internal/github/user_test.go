package github

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserID(t *testing.T) {
	var last *http.Request

	id, err := UserID(t.Context(), Credentials{Host: "github.com", Token: "tok"}, "rancher-scc-bot[bot]",
		respondWith(&last, http.StatusOK, `{"login":"rancher-scc-bot[bot]","id":583231}`))

	require.NoError(t, err)
	assert.Equal(t, int64(583231), id)
	require.NotNil(t, last, "the stub must have made a request")
	assert.Equal(t, "/users/rancher-scc-bot[bot]", last.URL.Path)
}

func TestUserIDUnknownLoginNamesIt(t *testing.T) {
	var last *http.Request

	_, err := UserID(t.Context(), Credentials{Host: "github.com", Token: "tok"}, "nobody-here",
		respondWith(&last, http.StatusNotFound, `{"message":"Not Found"}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nobody-here")
}
