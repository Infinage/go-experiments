package queens

import (
	"embed"
	"image"
	"reflect"
	"testing"

	_ "image/jpeg"
	_ "image/png"
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
			bbox:  image.Rect(251, 98, 662, 509),
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
			bbox:  image.Rect(670, 180, 1240, 748),
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
			fpath: "screenshots/2.jpg",
			bbox:  image.Rect(670, 180, 1240, 748),
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
			bbox:  image.Rect(263, 92, 675, 503),
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
		{
			fpath: "screenshots/5.png",
			bbox:  image.Rect(160, 153, 451, 444),
			grid: [][]int{
				{1, 1, 1, 2, 3, 4},
				{1, 1, 1, 2, 3, 4},
				{2, 2, 2, 2, 3, 4},
				{2, 2, 5, 3, 3, 4},
				{6, 5, 5, 3, 3, 4},
				{6, 6, 5, 5, 3, 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.fpath, func(t *testing.T) {
			f, err := testScreenshots.Open(tt.fpath)
			if err != nil {
				t.Fatal("Open fail: ", err)
			}

			img, format, err := image.Decode(f)
			f.Close()
			if err != nil {
				t.Fatalf("Image decode fail [%s]: %v", format, err)
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

			// Skip other checks in case the test is err only
			if tt.err != "" {
				return
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
		})
	}
}
