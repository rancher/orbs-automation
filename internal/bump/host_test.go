package bump

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	files map[string]string // "slug@ref:path" to content
	asked []string
}

func (f *fakeSource) UpstreamFile(_ context.Context, slug, ref, path string) ([]byte, error) {
	key := slug + "@" + ref + ":" + path
	f.asked = append(f.asked, key)
	content, ok := f.files[key]
	if !ok {
		return nil, errors.New("not found: " + key)
	}
	return []byte(content), nil
}

func TestHostTreeReadsTheFileOfTheRepositoryAtTheCommit(t *testing.T) {
	src := &fakeSource{files: map[string]string{"rancher/rancher@abc123:build.yaml": "image: x\n"}}

	got, err := HostTree(src, "rancher/rancher", "abc123").ReadFile(context.Background(), "build.yaml")

	require.NoError(t, err)
	assert.Equal(t, "image: x\n", string(got))
	assert.Equal(t, []string{"rancher/rancher@abc123:build.yaml"}, src.asked)
}

func TestHostTreeReportsAFileItCannotRead(t *testing.T) {
	_, err := HostTree(&fakeSource{}, "rancher/rancher", "abc123").ReadFile(context.Background(), "nope.yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope.yaml")
}
