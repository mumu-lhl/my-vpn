package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"
)

var (
	trayIconOnce         sync.Once
	trayIconConnected    []byte
	trayIconDisconnected []byte
)

// trayIconPNG returns a 32x32 PNG icon for the SNI tray.
// Both variants use monochrome black pixels with alpha so MyGo's template icon
// tinting adapts automatically to dark and light desktop panels.
func trayIconPNG(connected bool) []byte {
	trayIconOnce.Do(func() {
		trayIconConnected = renderTrayIconPNG(true)
		trayIconDisconnected = renderTrayIconPNG(false)
	})
	if connected {
		return trayIconConnected
	}
	return trayIconDisconnected
}

func renderTrayIconPNG(connected bool) []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size-1)/2.0, float64(size-1)/2.0

	for y := range size {
		for x := range size {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Hypot(dx, dy)

			// Outer ring represents the Looma relay loop.
			if dist >= 10.5 && dist <= 13.8 {
				img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 255})
				continue
			}

			if connected {
				// Solid inner core indicates an active tunnel.
				if dist <= 6.2 {
					img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 255})
				}
			} else {
				// Hollow inner ring indicates idle/disconnected state.
				if dist >= 3.8 && dist <= 5.8 {
					img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 220})
				}
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
