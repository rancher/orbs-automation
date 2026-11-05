package git

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
	}
	for _, tt := range tests {
		t.Run(tt.tag+" in "+tt.series, func(t *testing.T) {
			assert.Equal(t, tt.want, InSeries(tt.tag, tt.series))
		})
	}
}
