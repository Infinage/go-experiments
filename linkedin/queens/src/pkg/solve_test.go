package queens

import (
	"reflect"
	"testing"
)

func Test_BoardSolve(t *testing.T) {
	tests := []struct {
		name     string
		grid     [][]int
		solution [][]bool
		err      string
	}{
		{
			name: "7x7 valid",
			grid: [][]int{
				{1, 1, 1, 2, 3, 3, 3},
				{1, 1, 4, 2, 4, 3, 3},
				{1, 1, 4, 4, 4, 3, 3},
				{5, 5, 5, 4, 3, 3, 3},
				{5, 5, 5, 6, 3, 3, 3},
				{5, 7, 5, 6, 6, 3, 3},
				{5, 7, 7, 7, 6, 6, 3},
			},
			solution: [][]bool{
				{false, false, false, true, false, false, false},
				{true, false, false, false, false, false, false},
				{false, false, false, false, true, false, false},
				{false, false, true, false, false, false, false},
				{false, false, false, false, false, false, true},
				{false, true, false, false, false, false, false},
				{false, false, false, false, false, true, false},
			},
		},
		{
			name: "4x4 invalid",
			grid: [][]int{
				{1, 1, 1, 2},
				{5, 5, 5, 6},
				{5, 7, 5, 6},
				{5, 7, 7, 7},
			},
			err: "got 4 rows (want > 4)",
		},
		{
			name: "5x5 invalid",
			grid: [][]int{
				{1, 2, 2, 2, 2},
				{1, 1, 3, 4, 2},
				{1, 1, 3, 4, 2},
				{4, 4, 4, 4, 2},
				{5, 5, 4, 4, 2},
			},
			solution: [][]bool{
				{false, false, false, false, true},
				{false, false, true, false, false},
				{true, false, false, false, false},
				{false, false, false, true, false},
				{false, true, false, false, false},
			},
		},
		{
			name: "5x5 invalid",
			grid: [][]int{
				{1, 2, 2, 2, 1},
				{1, 1, 3, 4, 2},
				{1, 1, 3, 4, 2},
				{4, 4, 4, 4, 2},
				{5, 5, 4, 4, 2},
			},
			err: "no solution available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := Board{grid: tt.grid}
			if err := b.Solve(); tt.err == "" && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			} else if tt.err != "" && err == nil {
				t.Fatalf("Expected %q error, but passed", tt.err)
			} else if err != nil && tt.err != err.Error() {
				t.Fatalf("Expected %q error, got %q", tt.err, err.Error())
			}

			if tt.err != "" {
				return
			}

			if !reflect.DeepEqual(tt.solution, b.solution) {
				t.Errorf("Solution mismatched\nWant:\n%v\nGot:\n%v", tt.solution, b.solution)
			}
		})
	}
}
