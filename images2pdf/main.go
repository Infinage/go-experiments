package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/signintech/gopdf"
)

const A4Padding = 10

type ImagePlacement struct {
	Rotate90 bool
	Height   float64
	Width    float64
}

// getImagePlacement checks the optimal placement
// to make for a given image into an A4 page.
func getImagePlacement(imgW, imgH int, targetW, targetH float64) ImagePlacement {
	// Align larger of two sides against height
	scaleH := (targetH - 2*A4Padding) / float64(max(imgW, imgH))

	// Align smaller of two sides against width
	scaleW := (targetW - 2*A4Padding) / float64(min(imgW, imgH))

	// Actual scale needs to ensure that image still fits in
	scale := min(scaleH, scaleW)

	return ImagePlacement{
		Rotate90: imgW > imgH,
		Height:   float64(imgH) * scale,
		Width:    float64(imgW) * scale,
	}
}

func images2pdf(opath string, images []string) error {
	pdf := gopdf.GoPdf{}
	pageSize := *gopdf.PageSizeA4
	pdf.Start(gopdf.Config{PageSize: pageSize})

	for idx, fname := range images {
		f, err := os.Open(fname)
		if err != nil {
			return fmt.Errorf("read %q [#%d]: %w", fname, idx+1, err)
		}

		config, _, err := image.DecodeConfig(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("decode %q [#%d]: %w", fname, idx+1, err)
		}

		pdf.AddPage()
		placement := getImagePlacement(config.Width, config.Height,
			pageSize.W, pageSize.H)

		// Image height and width
		rect := &gopdf.Rect{W: placement.Width, H: placement.Height}

		if !placement.Rotate90 {
			err = pdf.Image(fname, A4Padding, A4Padding, rect)
			if err != nil {
				return fmt.Errorf("write %q [#%d]: %w", fname, idx+1, err)
			}
			continue
		}

		// Rotate at the center of the PDF
		centerX := pageSize.W / 2
		centerY := pageSize.H / 2
		pdf.Rotate(90, centerX, centerY)

		// Start drawing at the top left leaving as much space needed
		// for padding and for the image width itself
		offsetY := centerY - (pageSize.W / 2) + A4Padding
		offsetX := centerX + (pageSize.H / 2) - A4Padding - placement.Width

		if err = pdf.Image(fname, offsetX, offsetY, rect); err != nil {
			return fmt.Errorf("write %q [#%d]: %w", fname, idx+1, err)
		}

		// Reset rotation
		pdf.RotateReset()
	}

	// Flush all content into PDF
	pdf.WritePdf(opath)
	return nil
}

func main() {
	if len(os.Args) <= 2 {
		fmt.Println("Usage img2pdf <output> <image1> [<image2>, [<image3>, ...]]")
		return
	}

	if err := images2pdf(os.Args[1], os.Args[2:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
