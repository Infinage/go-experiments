package queens

import (
	"embed"
	"image"
	"image/png"
	"reflect"
	"testing"
)

//go:embed screenshots/*
var testScreenshots embed.FS

func Test_Detect(t *testing.T) {
	tests := []struct {
		fpath string
		bbox  image.Rectangle
		grid  [][]int
		err   string
	}{
		{
			fpath: "screenshots/1.png",
			bbox:  image.Rect(253, 100, 659, 507),
			grid: [][]int{
				{1, 1, 1, 1, 1, 1, 1},
				{1, 1, 1, 1, 1, 2, 2},
				{3, 2, 2, 2, 2, 2, 2},
				{3, 3, 3, 3, 3, 3, 4},
				{5, 5, 4, 4, 4, 4, 4},
				{5, 5, 5, 5, 6, 6, 6},
				{7, 7, 7, 7, 7, 6, 6},
			},
		},
		{
			fpath: "screenshots/2.png",
			bbox:  image.Rect(671, 182, 1232, 744),
			grid: [][]int{
				{1, 1, 1, 2, 2, 2},
				{1, 1, 1, 2, 2, 2},
				{1, 1, 1, 1, 3, 2},
				{4, 5, 1, 6, 3, 2},
				{4, 5, 1, 6, 6, 2},
				{4, 4, 1, 1, 1, 1},
			},
		},
		{
			fpath: "screenshots/3.png",
			bbox:  image.Rect(266, 94, 672, 500),
			grid: [][]int{
				{1, 1, 1, 2, 3, 3, 3},
				{1, 1, 4, 2, 4, 3, 3},
				{1, 1, 4, 4, 4, 3, 3},
				{5, 5, 5, 4, 3, 3, 3},
				{5, 5, 5, 6, 3, 3, 3},
				{5, 7, 5, 6, 6, 3, 3},
				{5, 7, 7, 7, 6, 6, 3},
			},
		},
		{
			fpath: "screenshots/4.png",
			err:   "no valid board detected",
		},
	}

	for _, tt := range tests {
		f, err := testScreenshots.Open(tt.fpath)
		if err != nil {
			t.Fatal("Open fail: %")
		}

		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal("PNG decode fail: ", err)
		}

		board, err := NewBoardFromImage(img)
		if tt.err == "" && err != nil {
			t.Fatal("Unexpected error: ", err)
		} else if tt.err != "" && err == nil {
			t.Log(board)
			t.Fatal("Expected error, but passed")
		} else if err != nil && tt.err != err.Error() {
			t.Fatalf("Expected error: %q, got %q", tt.err, err.Error())
		}

		// Assertion checks on the bounding box
		if want, got := tt.bbox.Dx(), board.bbox.Dx(); !almostEq(want, got, 5) {
			t.Errorf("bounding box size mismatched, want %d, got %d", want, got)
		}
		if want, got := tt.bbox.Min.X, board.bbox.Min.X; !almostEq(want, got, 5) {
			t.Errorf("start X mismatched, want %d, got %d", want, got)
		}
		if want, got := tt.bbox.Min.Y, board.bbox.Min.Y; !almostEq(want, got, 5) {
			t.Errorf("start Y mismatched, want %d, got %d", want, got)
		}

		// Validate grid matches every cell for cell
		if !reflect.DeepEqual(tt.grid, board.grid) {
			t.Errorf("Grid mismatched\nWant:\n%v\nGot:\n%v", tt.grid, board.grid)
		}
	}
}
