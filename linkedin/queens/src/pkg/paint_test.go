package queens

import (
	"image"
	"image/color"
	"testing"
)

func TestPaint(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 90, 90))

	b := &Board{
		img:  img,
		bbox: image.Rect(0, 0, 90, 90),
		solution: [][]bool{
			{true, false, false},
			{false, true, false},
			{false, false, true},
		},
	}

	if err := b.Paint(); err != nil {
		t.Fatalf("Paint() error: %v", err)
	}

	// Each cell is 30x30; centers are (15,15), (45,45) and (75,75)
	for _, p := range []image.Point{
		image.Pt(15, 15),
		image.Pt(45, 45),
		image.Pt(75, 75),
	} {
		got := img.RGBAAt(p.X, p.Y)
		if got.R != 255 || got.G != 0 || got.B != 0 || got.A != 255 {
			t.Errorf("expected red at %v, got %v", p, got)
		}
	}

	// The center of an unmarked cell should remain transparent.
	got := img.RGBAAt(45, 15)
	if got != (color.RGBA{}) {
		t.Errorf("expected transparent pixel at (45,15), got %v", got)
	}
}
