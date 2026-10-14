package queens

import "fmt"

// canPlace checks and returns true if a queen can be placed at provided cell.
func canPlace(row, col, color int, solution [][]bool, regions map[int][][2]int) bool {
	// Check for region
	_, ok := regions[color]
	if !ok {
		return false
	}

	// Check adjacent corners
	size := len(solution)
	for _, dy := range []int{-1, 1} {
		for _, dx := range []int{-1, 1} {
			x, y := row+dy, col+dx
			if x >= 0 && y >= 0 && x < size && y < size && solution[x][y] {
				return false
			}
		}
	}

	// Check for row
	for i := range size {
		if solution[row][i] {
			return false
		}
	}

	// Check for column
	for i := range size {
		if solution[i][col] {
			return false
		}
	}

	return true
}

// solve recursively attempts to solve the queens puzzle
func solve(row, col int, grid [][]int, solution [][]bool, regions map[int][][2]int) bool {
	size := len(solution)

	if row >= size {
		return true
	} else if col >= size {
		return false
	} else if !canPlace(row, col, grid[row][col], solution, regions) {
		return solve(row, col+1, grid, solution, regions)
	}

	// Place a queen at that point and try progressing forward
	solution[row][col] = true
	color := grid[row][col]
	region := regions[color]
	delete(regions, color)

	if solve(row+1, 0, grid, solution, regions) {
		regions[color] = region
		return true
	}

	solution[row][col] = false
	regions[color] = region
	return solve(row, col+1, grid, solution, regions)
}

// Solve derives a solution for the input queens puzzle and paints
// 'X's on image to denote queens placement. It returns an error if
// no solution is available.
func (b *Board) Solve() error {
	size := len(b.grid)
	if size <= 4 {
		fmt.Errorf("got %d rows (want > 4)", len(b.grid))
	}

	coloredRegions := make(map[int][][2]int)
	for rowIdx, row := range b.grid {
		if size != len(row) {
			return fmt.Errorf("got %d cols (want %d)", len(row), size)
		}

		for colIdx, cell := range row {
			coord := [2]int{rowIdx, colIdx}
			coloredRegions[cell] = append(coloredRegions[cell], coord)
		}
	}

	b.solution = make([][]bool, size)
	for idx := range b.solution {
		b.solution[idx] = make([]bool, size)
	}

	if !solve(0, 0, b.grid, b.solution, coloredRegions) {
		return fmt.Errorf("no solution available")
	}

	return nil
}
