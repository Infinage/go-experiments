package queens

import (
	"fmt"
	"testing"
)

func Test_BoardSolve(t *testing.T) {
	tests := []struct {
		grid     [][]int
		solution [][]bool
		err      string
	}{
		{
			grid: [][]int{
				{1, 1, 1, 2, 3, 3, 3},
				{1, 1, 4, 2, 4, 3, 3},
				{1, 1, 4, 4, 4, 3, 3},
				{5, 5, 5, 4, 3, 3, 3},
				{5, 5, 5, 6, 3, 3, 3},
				{5, 7, 5, 6, 6, 3, 3},
				{5, 7, 7, 7, 6, 6, 3},
			},
			solution: [][]bool {
				{false, false, false, true, false, false, false},
				{true, false, false, false, false, false, false},
				{false, false, false, false, true, false, false},
				{false, false, true, false, false, false, false},
				{false, false, false, false, false, false, true},
				{false, true, false, false, false, false, false},
				{false, false, false, false, false, true, false},
			},
		},
	}

	for _, tt := range tests {
		b := Board{grid: tt.grid}
		if err := b.Solve(); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		for _, row := range b.solution {
			for _, b := range row {
				val := 0
				if b {
					val = 1
				}
				fmt.Printf("%d ", val)
			}
			fmt.Println()
		}
		t.Fail()
	}
}
