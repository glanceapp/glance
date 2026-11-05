package glance

import "testing"

func TestSVGPolylineCoordsFromYValues(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   string
	}{
		{"empty", nil, ""},
		{"single value", []float64{10}, ""},
		{"increasing", []float64{10, 20, 30}, "0.00,49.00 50.00,25.00 100.00,1.00"},
		{"decreasing", []float64{30, 20, 10}, "0.00,1.00 50.00,25.00 100.00,49.00"},
		{"flat positive", []float64{42, 42, 42}, "0.00,25.00 50.00,25.00 100.00,25.00"},
		{"flat zero", []float64{0, 0}, "0.00,25.00 100.00,25.00"},
		{"flat negative", []float64{-5, -5}, "0.00,25.00 100.00,25.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svgPolylineCoordsFromYValues(100, 50, tt.values); got != tt.want {
				t.Errorf("svgPolylineCoordsFromYValues() = %q, want %q", got, tt.want)
			}
		})
	}
}
