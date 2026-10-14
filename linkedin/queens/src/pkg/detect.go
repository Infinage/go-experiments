package queens

import (
	"cmp"
	"fmt"
	"image"
	"math"
	"slices"
)

// imageCache stores pixels that have already been visited by isColoredEmptySquare
type imageCache map[[2]int]struct{}

// RegionStatus denotes the output returned by checkRegion
type RegionStatus int

const (
	RegionIn RegionStatus = iota
	RegionOut
	RegionDuplicate
)

type Board struct {
	img  image.Image
	bbox image.Rectangle
	grid [][]int
}

// almostEq returns true if values a and b are close to each
// other with the maximum specified tolerance. Panics if tol < 0.
func almostEq[T ~int | ~float32 | ~float64](a, b, tol T) bool {
	if tol < 0 {
		panic("negative tolerance")
	}

	delta := a - b
	if delta < 0 {
		delta = -delta
	}

	return delta <= tol
}

// centroid returns the mid point of the rectangle.
func centroid(rect image.Rectangle) image.Point {
	x := (rect.Max.X + rect.Min.X) / 2
	y := (rect.Max.Y + rect.Min.Y) / 2
	return image.Pt(x, y)
}

// isColoredEmptySquare checks how far a points color extends to the
// right and below. It must be a perfect square without any marks in between.
func isColoredEmptySquare(img image.Image, x, y int, cache imageCache) (int, bool) {
	r, g, b, _ := img.At(x, y).RGBA()
	bounds := img.Bounds()

	// Check column
	colSize := 0
	for y_ := y; y_ < bounds.Max.Y; y_++ {
		r_, g_, b_, _ := img.At(x, y_).RGBA()
		if r_ != r || g_ != g || b_ != b {
			break
		}
		cache[[2]int{x, y_}] = struct{}{}
		colSize++
	}

	// Check row
	rowSize := 0
	for x_ := x; x_ < bounds.Max.X; x_++ {
		r_, g_, b_, _ := img.At(x_, y).RGBA()
		if r_ != r || g_ != g || b_ != b {
			break
		}
		cache[[2]int{x_, y}] = struct{}{}
		rowSize++
	}

	// Sometimes borders can mess things up, a threshold is better. Also
	// regions smaller than 15 pixels are too tiny to be our regions of interest
	if !almostEq(rowSize, colSize, 3) || rowSize < 15 || colSize < 15 {
		return 0, false
	}

	// Check if square has same colored pixels uniformly
	// Some leeway since right bottom corner maybe have a border
	size := min(rowSize, colSize) - 3
	for dy := range size {
		for dx := range size {
			x_, y_ := x+dx, y+dy
			r_, g_, b_, _ := img.At(x_, y_).RGBA()
			if r_ != r || g_ != g || b_ != b {
				return 0, false
			}
			cache[[2]int{x_, y_}] = struct{}{}
		}
	}

	return size, true
}

// checkRegion compares a given rectangle against all rectangles in a region
// and returs a status code whether it should part of given region, not part of
// given region is a duplicate of another.
func checkRegion(rect image.Rectangle, region []image.Rectangle) RegionStatus {
	if len(region) == 0 {
		return RegionOut
	}

	c := centroid(rect)
	size := rect.Dx() // its a square

	var avgSize int
	var yOk, xOk bool
	for _, rr := range region {
		avgSize += rr.Dx()
		rc := centroid(rr)
		if almostEq(c.X, rc.X, 5) && almostEq(c.Y, rc.Y, 5) {
			return RegionDuplicate
		}
		if almostEq(c.X, rc.X, size+10) {
			xOk = true
		}
		if almostEq(c.Y, rc.Y, size+10) {
			yOk = true
		}
	}

	avgSize /= len(region)
	if yOk && xOk && almostEq(avgSize, size, 10) {
		return RegionIn
	}

	return RegionOut
}

// getBoundary returns the absolute min and max boundaries
func getBoundary(region []image.Rectangle) image.Rectangle {
	start := image.Pt(math.MaxInt, math.MaxInt)
	end := image.Pt(0, 0)
	for _, rect := range region {
		if rect.Min.X < start.X {
			start.X = rect.Min.X
		}
		if rect.Min.Y < start.Y {
			start.Y = rect.Min.Y
		}
		if rect.Max.X > end.X {
			end.X = rect.Max.X
		}
		if rect.Max.Y > end.Y {
			end.Y = rect.Max.Y
		}
	}
	return image.Rect(start.X, start.Y, end.X, end.Y)
}

// constructGrid cleans the detected square cells and constructs the textual
// representation of the nqueens board.
func constructGrid(img image.Image, region []image.Rectangle, rows int) [][]int {
	slices.SortFunc(region, func(a, b image.Rectangle) int {
		if almostEq(a.Min.Y, b.Min.Y, 3) {
			return cmp.Compare(a.Min.X, b.Min.X)
		}
		return cmp.Compare(a.Min.Y, b.Min.Y)
	})

	// Initialize grid and color map
	grid := make([][]int, rows)
	for idx := range grid {
		grid[idx] = make([]int, rows)
	}

	// temporary struct to hash
	type RGB struct{ r, g, b uint32 }

	var row, col int
	colors := make(map[RGB]int)
	for _, rect := range region {
		c := centroid(rect)
		r, g, b, _ := img.At(c.X, c.Y).RGBA()
		key := RGB{r, g, b}
		if _, ok := colors[key]; !ok {
			colors[key] = len(colors) + 1
		}

		grid[row][col] = colors[key]
		col++
		if col == rows {
			col = 0
			row++
		}
	}

	return grid
}

// DetectBoard parses the input image and returns a struct containing detected
// queens board along with the textual representation of it.
func DetectBoard(img image.Image) (Board, error) {
	cache := make(imageCache)
	var regions [][]image.Rectangle

	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if _, ok := cache[[2]int{x, y}]; !ok {
				size, ok := isColoredEmptySquare(img, x, y, cache)
				if !ok {
					continue
				}

				var processed bool
				rect := image.Rect(x, y, x+size, y+size)

			loop:
				for idx := range regions {
					switch checkRegion(rect, regions[idx]) {
					// Merge into existing region
					case RegionIn:
						regions[idx] = append(regions[idx], rect)
						fallthrough

					// If duplicate, skip merging or inserting fresh
					case RegionDuplicate:
						processed = true
						break loop
					}
				}

				if !processed { // Part of a new region
					regions = append(regions, []image.Rectangle{rect})
				}
			}
		}
	}

	type largestRegion struct{ idx, totalSize, rows int }
	var largest largestRegion

	// Find the largest square shaped region seen so far
	for idx, region := range regions {
		N := len(region)
		sqrt := int(math.Sqrt(float64(N)))
		if sqrt*sqrt != N {
			continue
		}

		if tsize := region[0].Dx() * sqrt; tsize > largest.totalSize {
			largest = largestRegion{idx, tsize, sqrt}
		}
	}

	if largest.rows <= 4 {
		return Board{}, fmt.Errorf("no valid board detected")
	}

	region := slices.Clone(regions[largest.idx])
	grid := constructGrid(img, region, largest.rows)
	board := Board{img: img, grid: grid, bbox: getBoundary(region)}

	distinct := make(map[int]struct{})
	for _, row := range board.grid {
		for _, cell := range row {
			distinct[cell] = struct{}{}
		}
	}

	if len(distinct) != largest.rows {
		return Board{}, fmt.Errorf("no valid board detected")
	}

	return board, nil
}
