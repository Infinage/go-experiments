package main

import (
	"fmt"
	"image"

	"github.com/signintech/gopdf"
)

const A4Padding = 10

// ImagePlacement represents the best possible orientation / scale
// required to minimize whitespace.
type ImagePlacement struct {
	Rotate90 bool
	Height   float64
	Width    float64
}

// getImagePlacement checks the optimal placement to make for a given image into an A4 page.
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

// images2pdf takes in a list of images and determines best possible
// `ImagePlacement` to minimize the whitespace. Each image is placed
// on a seperate A4 sheet.
func images2pdf(images []image.Image) (*gopdf.GoPdf, error) {
	pdf := gopdf.GoPdf{}
	pageSize := *gopdf.PageSizeA4
	pdf.Start(gopdf.Config{PageSize: pageSize})

	for idx, img := range images {
		bounds := img.Bounds()

		pdf.AddPage()
		placement := getImagePlacement(bounds.Dx(), bounds.Dy(),
			pageSize.W, pageSize.H)

		// Image height and width
		rect := &gopdf.Rect{W: placement.Width, H: placement.Height}

		if !placement.Rotate90 {
			err := pdf.ImageFrom(img, A4Padding, A4Padding, rect)
			if err != nil {
				return nil, fmt.Errorf("write #%d: %w", idx+1, err)
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

		if err := pdf.ImageFrom(img, offsetX, offsetY, rect); err != nil {
			return nil, fmt.Errorf("write #%d: %w", idx+1, err)
		}

		// Reset rotation
		pdf.RotateReset()
	}

	return &pdf, nil
}
