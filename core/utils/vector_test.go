package utils

import (
	"math"
	"testing"
)

const eps = 1e-9

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= eps
}

func TestCosine(t *testing.T) {
	tests := []struct {
		name string
		a    []float64
		b    []float64
		want float64
	}{
		{
			name: "len_mismatch_returns_zero",
			a:    []float64{1, 2},
			b:    []float64{1},
			want: 0,
		},
		{
			name: "empty_vectors_return_zero",
			a:    []float64{},
			b:    []float64{},
			want: 0,
		},
		{
			name: "zero_norm_returns_zero",
			a:    []float64{0, 0, 0},
			b:    []float64{1, 2, 3},
			want: 0,
		},
		{
			name: "identical_vectors_are_one",
			a:    []float64{1, 2, 3},
			b:    []float64{1, 2, 3},
			want: 1,
		},
		{
			name: "orthogonal_vectors_are_zero",
			a:    []float64{1, 0},
			b:    []float64{0, 1},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Cosine(tt.a, tt.b)
			if !almostEqual(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestExpDecay(t *testing.T) {
	tests := []struct {
		name   string
		lambda float64
		t      float64
		want   float64
	}{
		{
			name:   "t_zero_returns_one",
			lambda: 0.7,
			t:      0,
			want:   1,
		},
		{
			name:   "lambda_zero_returns_one",
			lambda: 0,
			t:      3.5,
			want:   1,
		},
		{
			name:   "regular_decay",
			lambda: 0.5,
			t:      2,
			want:   math.Exp(-1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpDecay(tt.lambda, tt.t)
			if !almostEqual(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
