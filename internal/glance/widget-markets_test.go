package glance

import "testing"

// Yahoo sometimes returns null for a daily close (e.g. the previous day's bar
// for forex symbols such as EUR=X), which gets decoded as 0.
func TestPreviousMarketClose(t *testing.T) {
	const fallback = 0.8911

	tests := []struct {
		name   string
		prices []float64
		want   float64
	}{
		{"previous close is null", []float64{0.88861, 0.89275, 0, 0.8911}, 0.89275},
		{"multiple nulls before current", []float64{0.88861, 0, 0, 0.8911}, 0.88861},
		{"no nulls", []float64{85557.5625, 83275.9296875, 81676.3359375, 82631.828125}, 81676.3359375},
		{"current close is null", []float64{0.88861, 0.89275, 0}, 0.89275},
		{"single value", []float64{0.8911}, fallback},
		{"all zeros", []float64{0, 0, 0, 0}, fallback},
		{"empty", []float64{}, fallback},
	}

	for _, tt := range tests {
		if got := previousMarketClose(tt.prices, fallback); got != tt.want {
			t.Errorf("%s: previousMarketClose(%v) = %v, want %v", tt.name, tt.prices, got, tt.want)
		}
	}
}
