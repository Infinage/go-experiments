package queens

import (
	"fmt"
	"image"
	"image/color"
	"slices"

	"github.com/anthonynsimon/bild/effect"
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
	img      image.Image
	bbox     image.Rectangle
	grid     [][]int
	solution [][]bool
}

// almostEq returns true if values a and b are close to each
// other with the maximum specified tolerance. Panics if tol < 0.
func almostEq[T ~uint32 | ~int | ~float32 | ~float64](a, b, tol T) bool {
	if tol < 0 {
		panic("negative tolerance")
	}

	if a > b {
		return a-b <= tol
	}

	return b-a <= tol
}

// centroid returns the mid point of the rectangle.
func centroid(rect image.Rectangle) image.Point {
	x := (rect.Max.X + rect.Min.X) / 2
	y := (rect.Max.Y + rect.Min.Y) / 2
	return image.Pt(x, y)
}

// compactAverage compacts the slice by comparing each element using `almostEq`.
// The elements themselves are averaged. Assumes values are already sorted.
func compactAverage(values []int, threshold int) []int {
	if len(values) == 0 {
		return values
	}

	var result []int
	var currentTotal, count int
	for idx := range values {
		if idx == 0 {
			currentTotal, count = values[idx], 1
			continue
		}

		// On encountering a new element, insert existing one into result
		if !almostEq(currentTotal/count, values[idx], threshold) {
			result = append(result, currentTotal/count)
			currentTotal, count = values[idx], 1
			continue
		}

		// Can be compacted, update the running total values
		currentTotal += values[idx]
		count++
	}

	result = append(result, currentTotal/count)
	return result
}

// cleanGaps assumes each line x line as a potential 'gap'.
// Assumes that values is already sorted.
func cleanGaps(values []int) []int {
	// Cache gaps and how many edges each gap accounts for
	gapsMap := make(map[int]int)
	for i := range values {
		for j := i + 1; j < len(values); j++ {
			var accountsFor int
			gap := values[j] - values[i]
			for k := range len(values) - j {
				target := values[j] + (gap * k)
				pos, ok := slices.BinarySearch(values, target)
				if ok ||
					(pos > 0 && almostEq(values[pos-1], target, 5)) ||
					(pos < len(values) && almostEq(values[pos], target, 5)) {
					accountsFor++
				}
			}
			gapsMap[gap] += accountsFor
		}
	}

	// Merge the gaps and find the most commonly occuring gap
	var gapItems [][2]int
	for gap, count := range gapsMap {
		gapItems = append(gapItems, [2]int{gap, count})
	}
	slices.SortFunc(gapItems, func(a, b [2]int) int { return a[0] - b[0] })

	// Merge the gap items, use almostEq for comparison
	var mergedGapItems [][2]int
	var currentTotal, currentCount, frequency int
	for idx := range gapItems {
		gi := gapItems[idx]

		if idx == 0 {
			currentTotal, currentCount, frequency = gi[0], 1, gi[1]
			continue
		}

		if avg := currentTotal / currentCount; !almostEq(avg, gi[0], 5) {
			mergedGapItems = append(mergedGapItems, [2]int{avg, frequency})
			currentTotal, currentCount, frequency = gi[0], 1, gi[1]
			continue
		}

		currentCount++
		currentTotal += gi[0]
		frequency += gi[1]
	}

	// Append any leftovers
	if len(gapItems) > 0 {
		mergedGapItems = append(mergedGapItems, [2]int{currentTotal / currentCount,
			frequency})
	}

	// Possible that that no merge candiates were available
	if len(mergedGapItems) == 0 {
		return nil
	}

	// Most commonly occuring gap
	gap := slices.MaxFunc(mergedGapItems, func(a, b [2]int) int { return a[1] - b[1] })[0]

	// Find the longest contiguous sequence that match the required gap
	var bestSequence []int
	for i := range values {
		curr := values[i]
		seq := []int{curr}
		for j := i + 1; j < len(values); j++ {
			if almostEq(values[j], curr+gap, 5) {
				seq = append(seq, values[j])
				curr = values[j]
			}
		}
		if len(seq) > len(bestSequence) {
			bestSequence = seq
		}
	}

	return bestSequence
}

// newGridFromImage constructs textual representation of the grid
// given the coordinates.
func newGridFromImage(img image.Image, xs, ys []int) ([][]int, error) {
	// Init empty grid (edges - 1)
	N := len(xs)
	grid := make([][]int, N-1)
	for i := range grid {
		grid[i] = make([]int, N-1)
	}

	colors := make(map[color.Color]int)
	for j := range N - 1 {
		for i := range N - 1 {
			xm := (xs[i] + xs[i+1]) / 2
			ym := (ys[j] + ys[j+1]) / 2

			key, ok := colors[img.At(xm, ym)]
			if !ok {
				key = len(colors) + 1
				colors[img.At(xm, ym)] = key
			}

			grid[j][i] = key
		}
	}

	if len(colors) != N-1 {
		return nil, fmt.Errorf("colors [%d] != grid dimensions [%d]",
			len(colors), N-1)
	}

	return grid, nil
}

// NewBoardFromImage parses the input image and returns a struct containing detected
// queens board along with the textual representation of it.
func NewBoardFromImage(img image.Image) (Board, error) {
	imgSobel := effect.Sobel(img)
	bounds := imgSobel.Bounds()

	xm := make(map[int]int)
	ym := make(map[int]int)

	// Conver pixel to grayscale and count pixels greater than threshold
	intensityThreshold := uint8(180)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := imgSobel.At(x, y).RGBA()

			// Convert the 16-bit down to standard 8-bit
			m := max(r, g, b) >> 8

			// If it clears the threshold, save coordinates
			if uint8(m) > intensityThreshold {
				xm[x]++
				ym[y]++
			}
		}
	}

	// We consider lines that are atleast 'lengthThreshold' pixels wide
	lengthThreshold := 250
	var xs, ys []int
	for x, count := range xm {
		if count > lengthThreshold {
			xs = append(xs, x)
		}
	}
	for y, count := range ym {
		if count > lengthThreshold {
			ys = append(ys, y)
		}
	}

	// Sort the lines
	slices.Sort(xs)
	slices.Sort(ys)

	// Merge (average) lines that are close to each other
	proximityThreshold := 5
	xs = compactAverage(xs, proximityThreshold)
	ys = compactAverage(ys, proximityThreshold)

	// Do a gap analysis and prune out edges that cannot be explained
	xs, ys = cleanGaps(xs), cleanGaps(ys)
	if len(xs) == 0 || len(ys) == 0 {
		return Board{}, fmt.Errorf("no valid board detected")
	}

	// Queens is a square shaped puzzle, in case of size differences
	// add edge shorted axis based on the direction with higher intensity
	switch {
	case len(xs) < len(ys):
		gap := xs[1] - xs[0]
		left, right := xs[0]-gap, xs[len(xs)-1]+gap
		if xm[left] > xm[right] {
			xs = append([]int{left}, xs...)
		} else {
			xs = append(xs, right)
		}

	case len(xs) > len(ys):
		gap := ys[1] - ys[0]
		top, bottom := ys[0]-gap, ys[len(ys)-1]+gap
		if ym[top] > ym[bottom] {
			ys = append([]int{top}, ys...)
		} else {
			ys = append(ys, bottom)
		}
	}

	// Length difference greater than one edge
	if len(xs) != len(ys) {
		return Board{}, fmt.Errorf("no valid board detected")
	}

	grid, err := newGridFromImage(img, xs, ys)
	if err != nil {
		return Board{}, fmt.Errorf("newGridFromImage: %w", err)
	}

	bbox := image.Rect(xs[0], ys[0], xs[len(xs)-1], ys[len(ys)-1])

	return Board{img: img, bbox: bbox, grid: grid}, nil
}
