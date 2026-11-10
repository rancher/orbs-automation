package lines

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntersectIsTheLinesBothRangesHold(t *testing.T) {
	tests := []struct {
		name string
		a, b Range
		want Range
	}{
		{"both open", Range{}, Range{}, Range{}},
		{"one open takes the other", Range{}, Range{Since: "v2.14", Until: "v2.16"}, Range{Since: "v2.14", Until: "v2.16"}},
		{"the later start and the earlier end", Range{Since: "v2.13", Until: "v2.16"}, Range{Since: "v2.14", Until: "v2.15"}, Range{Since: "v2.14", Until: "v2.15"}},
		{"a start of one and an end of the other", Range{Since: "v2.15"}, Range{Until: "v2.16"}, Range{Since: "v2.15", Until: "v2.16"}},
		{"written with and without the v", Range{Since: "2.15"}, Range{Since: "v2.14"}, Range{Since: "2.15"}},
		{"the same line", Range{Since: "v2.15", Until: "v2.15"}, Range{Since: "v2.15"}, Range{Since: "v2.15", Until: "v2.15"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.a.Intersect(tt.b))
			assert.Equal(t, tt.want, tt.b.Intersect(tt.a), "in either order")
		})
	}
}

func TestIntersectOfRangesThatShareNoLineDoesNotOverlap(t *testing.T) {
	a, b := Range{Until: "v2.14"}, Range{Since: "v2.15"}

	assert.False(t, a.Overlaps(b))
	assert.False(t, a.Intersect(b).Overlaps(a), "what is left holds no line either range holds")
}
