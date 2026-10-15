package queens

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

// markX draws an X of given size at specified coordinates, thickness of 2 pixels
func markX(img draw.Image, x, y, size, thickness int) error {
	bounds := img.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X ||
		y < bounds.Min.Y || y >= bounds.Max.Y {
		return fmt.Errorf("(%d,%d) out of bounds [%v]", x, y, bounds)
	}

	red := image.NewUniform(color.RGBA{255, 0, 0, 255})
	for i := -size / 2; i <= size/2; i++ {
		// Top-Left to Bottom-Right (\)
		tl2br := image.Rect(x+i, y+i, x+i+thickness, y+i+thickness)
		draw.Draw(img, tl2br, red, image.Point{}, draw.Src)

		// Bottom-Left to Top-Right (/)
		bl2tr := image.Rect(x+i, y-i, x+i+thickness, y-i+thickness)
		draw.Draw(img, bl2tr, red, image.Point{}, draw.Src)
	}

	return nil
}

// Paint returns the cropped game board image from input and draws
// 'X' on points where queen can be safely placed.
func (b *Board) Paint() (image.Image, error) {
	if b.img == nil {
		return nil, fmt.Errorf("image not set")
	}

	// Create a copy of the original image.
	canvas := image.NewRGBA(image.Rect(0, 0, b.bbox.Dx(), b.bbox.Dy()))
	draw.Draw(canvas, canvas.Bounds(), b.img, b.bbox.Min, draw.Src)

	size := len(b.solution)
	if size == 0 {
		return nil, fmt.Errorf("not solved yet")
	}

	boxSize := b.bbox.Dx() / size
	for rowIdx, row := range b.solution {
		for colIdx, marked := range row {
			if marked {
				cx := colIdx*boxSize + boxSize/2
				cy := rowIdx*boxSize + boxSize/2
				if err := markX(canvas, cx, cy, boxSize/3, 3); err != nil {
					return nil, fmt.Errorf("mark [%d, %d]: %w", rowIdx, colIdx, err)
				}
			}
		}
	}

	return canvas, nil
}
