package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInSeries(t *testing.T) {
	tests := []struct {
		tag, series string
		want        bool
	}{
		{"v2.15.0", "v2.15", true},
		{"v2.15.3-rc.1", "v2.15", true},
		{"v2.15.3", "2.15", true},
		{"v2.16.0", "v2.15", false},
		{"v2.150.0", "v2.15", false},
		{"v2.1.0", "v2.15", false},
		{"v9.4.1", "9.x", true},
		{"v9.0.0-rc.1", "9.x", true},
		{"v10.0.0", "9.x", false},
		{"v8.1.2", "9.x", false},
		{"v2.15.3", "v2.15.3", true},
		{"v2.15.4", "v2.15.3", false},
		{"v2.15.3", "v2.x.3", true},
		{"v2.15.3", "x", true},
		{"main-abc1234", "v2.15", false},
		{"v2.15.3", "latest", false},
		{"v2.15.3", "", false},
		{"v2.15.3", "v2.15.3.1", false},
		{"111.0.2+up1.2.3", "111.x", true},
	}
	for _, tt := range tests {
		t.Run(tt.tag+" in "+tt.series, func(t *testing.T) {
			assert.Equal(t, tt.want, InSeries(tt.tag, tt.series))
		})
	}
}

func TestParseSeries(t *testing.T) {
	for series, want := range map[string]string{
		"9.x": "9.x", "0.9.x": "0.9.x", "v2.15": "2.15", "2.x.3": "2.x.3", "x": "x", "111.x": "111.x",
	} {
		got, ok := ParseSeries(series)

		assert.True(t, ok, series)
		assert.Equal(t, want, got.String(), series)
	}
	for _, series := range []string{"", "latest", "1.2.3.4", "1..x", "-1.x", "1.y", "v"} {
		_, ok := ParseSeries(series)

		assert.False(t, ok, "%q", series)
	}
}

func TestSeriesContainsAVersion(t *testing.T) {
	v, ok := Parse("v0.9.4-rc.1")
	assert.True(t, ok)

	for series, want := range map[string]bool{"0.x": true, "0.9.x": true, "0.9.4": true, "0.10.x": false, "1.x": false, "0.9.5": false} {
		s, ok := ParseSeries(series)
		assert.True(t, ok, series)
		assert.Equal(t, want, s.Contains(v), series)
	}
}

func TestSeriesOverlap(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"9.x", "9.x", true},
		{"9.x", "8.x", false},
		{"0.9.x", "0.8.x", false},
		{"0.9.x", "0.9.x", true},
		{"0.x", "0.9.x", true},
		{"0.9.x", "0.x", true},
		{"0.x", "1.x", false},
		{"2.x.3", "2.1.x", true},
		{"2.x.3", "2.1.4", false},
		{"x", "7.x", true},
	}
	for _, tt := range tests {
		t.Run(tt.a+" and "+tt.b, func(t *testing.T) {
			a, ok := ParseSeries(tt.a)
			assert.True(t, ok)
			b, ok := ParseSeries(tt.b)
			assert.True(t, ok)

			assert.Equal(t, tt.want, a.Overlaps(b))
			assert.Equal(t, tt.want, b.Overlaps(a), "it does not matter which is asked")
		})
	}
}
