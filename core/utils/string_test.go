package utils

import (
	"slices"
	"testing"
)

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantNil bool
	}{
		{
			name:    "dedupe_preserves_order_and_filters_empty",
			in:      []string{"alpha", "", "beta", "alpha", "", "gamma", "beta"},
			want:    []string{"alpha", "beta", "gamma"},
			wantNil: false,
		},
		{
			name:    "all_empty",
			in:      []string{"", "", ""},
			want:    []string{},
			wantNil: true,
		},
		{
			name:    "already_unique",
			in:      []string{"one", "two", "three"},
			want:    []string{"one", "two", "three"},
			wantNil: false,
		},
		{
			name:    "nil_input",
			in:      nil,
			want:    []string(nil),
			wantNil: true,
		},
		{
			name:    "single_value_repeated",
			in:      []string{"z", "z", "z"},
			want:    []string{"z"},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UniqueStrings(tt.in)

			if (got == nil) != tt.wantNil {
				t.Fatalf("nilness: got nil=%t want nil=%t", got == nil, tt.wantNil)
			}

			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
