package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

func makeTrayIcon() []byte {
	icon := image.NewRGBA(image.Rect(0, 0, 32, 32))
	draw.Draw(icon, icon.Bounds(), &image.Uniform{C: color.RGBA{12, 29, 45, 255}}, image.Point{}, draw.Src)
	for y := 4; y < 28; y++ {
		for x := 4; x < 28; x++ {
			dx, dy := x-16, y-16
			if dx*dx+dy*dy <= 12*12 {
				icon.SetRGBA(x, y, color.RGBA{79, 209, 180, 255})
			}
		}
	}
	// Small dark center makes the icon recognizable at the tray's native size.
	for y := 11; y < 21; y++ {
		for x := 11; x < 21; x++ {
			icon.SetRGBA(x, y, color.RGBA{12, 29, 45, 255})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, icon); err != nil {
		return nil
	}
	return output.Bytes()
}
