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

	oimg, err := b.Paint()
	if err != nil {
		t.Fatalf("Paint() error: %v", err)
	}

	// The returned image should be cropped to the board bounds.
	if got, want := oimg.Bounds(), b.bbox; got != want {
		t.Fatalf("Paint() bounds = %v, want %v", got, want)
	}

	// The marked cell centers should be red in the output image.
	red := color.RGBA{255, 0, 0, 255}
	for _, p := range []image.Point{
		image.Pt(15, 15),
		image.Pt(45, 45),
		image.Pt(75, 75),
	} {
		got := color.RGBAModel.Convert(oimg.At(p.X, p.Y))
		if got != red {
			t.Errorf("expected red at %v, got %v", p, got)
		}
	}

	// An unmarked cell's center should remain transparent.
	got := color.RGBAModel.Convert(oimg.At(45, 15))
	if got != (color.RGBA{}) {
		t.Errorf("expected transparent pixel at (45,15), got %v", got)
	}

	// Paint() must not modify the original image.
	for _, p := range []image.Point{
		image.Pt(15, 15),
		image.Pt(45, 45),
		image.Pt(75, 75),
		image.Pt(45, 15),
	} {
		got := img.RGBAAt(p.X, p.Y)
		if got != (color.RGBA{}) {
			t.Errorf("original image modified at %v: got %v", p, got)
		}
	}
}
