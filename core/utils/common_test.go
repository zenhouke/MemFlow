package utils

import "testing"

func TestToFloat32(t *testing.T) {
	tests := []struct {
		name string
		in   []float64
		want []float32
	}{
		{name: "empty", in: []float64{}, want: []float32{}},
		{name: "normal", in: []float64{1.5, -2.25, 0}, want: []float32{1.5, -2.25, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToFloat32(tt.in)

			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("idx %d: got %v want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		name string
		a    float64
		b    float64
		want float64
	}{
		{name: "a_greater", a: 3, b: 2, want: 3},
		{name: "b_greater", a: 2, b: 3, want: 3},
		{name: "equal", a: 2, b: 2, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Max(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name string
		a    float64
		b    float64
		want float64
	}{
		{name: "a_less", a: 2, b: 3, want: 2},
		{name: "b_less", a: 3, b: 2, want: 2},
		{name: "equal", a: 2, b: 2, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Min(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
