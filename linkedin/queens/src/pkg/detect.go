package queens

import (
	"fmt"
	"image"
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
				target := values[j]+(gap*k)
				pos, ok := slices.BinarySearch(values, target)
				if ok || 
					(pos > 0 && almostEq(values[pos - 1], target, 5)) || 
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
	mergedGapItems = append(mergedGapItems, [2]int{currentTotal / currentCount,
		frequency})

	// Most commonly occuring gap
	gap := slices.MaxFunc(mergedGapItems, func(a, b [2]int) int { return a[1] - b[1] })[0]

	// Determine the starting candidate for the given gap
	bestCandidate := [2]int{-1, 0}
	for i := range values {
		var score int
		for j := range len(values) - i {
			target := values[i]+(gap*j)
			pos, ok := slices.BinarySearch(values, target)
			if ok ||
				(pos > 0 && almostEq(values[pos - 1], target, 5)) || 
				(pos < len(values) && almostEq(values[pos], target, 5)) {
				score++
			}
		}
		if bestCandidate[1] < score {
			bestCandidate = [2]int{i, score}
		}
	}

	// Remove edges that do not confirm
	start, keep := values[bestCandidate[0]], 0
	for i := range values {
		mod := (values[i] - start) % gap
		if mod < 0 {
			mod += gap
		}
		if mod < 10 || gap-mod < 10 {
			values[keep] = values[i]
			keep++
		}
	}

	values = values[:keep]
	return values
}

// NewBoardFromImage parses the input image and returns a struct containing detected
// queens board along with the textual representation of it.
func NewBoardFromImage(img image.Image) (Board, error) {
	imgSobel := effect.Sobel(img)
	bounds := imgSobel.Bounds()

	xm := make(map[int]int)
	ym := make(map[int]int)

	// Conver pixel to grayscale and count pixels greater than threshold
	intensityThreshold := uint8(128)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := imgSobel.At(x, y).RGBA()

			// Convert the 16-bit down to standard 8-bit
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)

			// Calculate standard grayscale luma intensity
			gray := uint8(0.299*float64(r8) + 0.587*float64(g8) + 0.114*float64(b8))

			// If it clears the threshold, save coordinates
			if gray > intensityThreshold {
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

	for _, x := range xs {
		fmt.Println(x)
	}
	fmt.Println()
	for _, y := range ys {
		fmt.Println(y)
	}

	return Board{}, nil
}
