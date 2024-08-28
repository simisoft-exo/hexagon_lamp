package lights

import (
	"fmt"
	"log"
	"time"

	"github.com/fogleman/gg"
)

type RGBW struct {
	R, G, B, W uint8
}

func (c RGBW) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	g = uint32(c.G)
	b = uint32(c.B)
	w := uint32(c.W)
	// Combine RGB and W, you might need to adjust this based on your LED characteristics
	r = (r*255 + w*255) / 255
	g = (g*255 + w*255) / 255
	b = (b*255 + w*255) / 255
	a = 0xffff
	return
}

func DrawHexagon(dc *gg.Context, x, y, size float64, c RGBW) {
	dc.SetColor(c)
	dc.DrawRegularPolygon(6, x, y, size, 0)
	dc.Fill()
}

const (
	PI               = 3.14159265358979323846
	HEX_HEIGHT_RATIO = 0.8128988125
	LUT_W            = 11
	LUT_H            = 12
	LUT_LEN          = LUT_W * LUT_H
)

var LUT = [LUT_LEN]int{
	-1, -1, -1, 0, -1, 1, -1, 2, -1, -1, -1,
	-1, -1, 6, -1, 5, -1, 4, -1, 3, -1, -1,
	-1, -1, 7, -1, 8, -1, 9, -1, 10, -1, -1,
	-1, 15, -1, 14, -1, 13, -1, 12, -1, 11, -1,
	-1, 16, -1, 17, -1, 18, -1, 19, -1, 20, -1,
	26, -1, 25, -1, 24, -1, 23, -1, 22, -1, 21,
	27, -1, 28, -1, 29, -1, 30, -1, 31, -1, 32,
	-1, 37, -1, 36, -1, 35, -1, 34, -1, 33, -1,
	-1, 38, -1, 39, -1, 40, -1, 41, -1, 42, -1,
	-1, -1, 46, -1, 45, -1, 44, -1, 43, -1, -1,
	-1, -1, 47, -1, 48, -1, 49, -1, 50, -1, -1,
	-1, -1, -1, 53, -1, 52, -1, 51, -1, -1, -1,
}

type HexagonPanel struct {
	Width  int
	Height int
	Leds   []RGBW
	Driver *LEDDriver
}

func NewHexagonPanel() (*HexagonPanel, error) {
	driver, err := makeDriver()
	if err != nil {
		return nil, fmt.Errorf("failed to create LED driver: %w", err)
	}

	return &HexagonPanel{
		Width:  LUT_W,
		Height: LUT_H,
		Leds:   make([]RGBW, 54), // 54 is the highest LED index in the LUT + 1
		Driver: driver,
	}, nil
}

func RunAnimations() {

	panel, err := NewHexagonPanel()
	if err != nil {
		log.Fatalf("Error creating hexagon panel: %v", err)
	}

	for {
		RunGrowingShrinkingHexagon(panel)
		// Add more animations here if needed
	}
}

func RunGrowingShrinkingHexagon(panel *HexagonPanel) {
	maxSize := float64(LUT_W) / 2 // Maximum size of the hexagon

	for {
		for size := 0.0; size <= maxSize; size += 0.1 {
			renderHexagonFrame(panel, size)
		}

		for size := maxSize; size >= 0; size -= 0.1 {
			renderHexagonFrame(panel, size)
		}
	}
}

func renderHexagonFrame(panel *HexagonPanel, size float64) {
	dc := gg.NewContext(LUT_W, LUT_H)
	dc.SetRGB(0, 0, 0) // Set background to black
	dc.Clear()

	// Draw the hexagon
	centerX, centerY := float64(LUT_W)/2, float64(LUT_H)/2
	DrawHexagon(dc, centerX, centerY, size, RGBW{R: 255, G: 0, B: 0, W: 0}) // Red hexagon

	// Convert the drawing to LED data
	panel.DrawToPanel(dc)

	// Convert RGBW to uint32 for the LED driver
	ledData := make([]uint32, len(panel.Leds))
	for i, led := range panel.Leds {
		ledData[i] = uint32(led.R)<<16 | uint32(led.G)<<8 | uint32(led.B)
	}

	// Render the frame
	if err := panel.Driver.Render(ledData); err != nil {
		log.Printf("Error rendering frame: %v", err)
	}

	time.Sleep(50 * time.Millisecond) // Adjust speed as needed
}

func (h *HexagonPanel) DrawToPanel(dc *gg.Context) {
	for y := 0; y < h.Height; y++ {
		for x := 0; x < h.Width; x++ {
			index := LUT[y*h.Width+x]
			if index != -1 {
				r, g, b, a := dc.Image().At(x, y).RGBA()
				h.Leds[index] = RGBW{
					R: uint8(r >> 8),
					G: uint8(g >> 8),
					B: uint8(b >> 8),
					W: uint8(a >> 8), // Using alpha value for W
				}
			}
		}
	}
}

func Run() {
	panel, err := NewHexagonPanel()
	if err != nil {
		log.Fatalf("Error creating hexagon panel: %v", err)
	}
	// Define animation functions
	animations := []func(*HexagonPanel){
		func(panel *HexagonPanel) {
			// Original animation
			for i := 0; i < len(panel.Leds); i++ {
				panel.Leds[i] = RGBW{R: 255, G: 255, B: 255, W: 255}
				ledData := make([]uint32, len(panel.Leds))
				for j, led := range panel.Leds {
					ledData[j] = uint32(led.W)<<24 | uint32(led.R)<<16 | uint32(led.G)<<8 | uint32(led.B)
				}
				if err := panel.Driver.Render(ledData); err != nil {
					log.Printf("Error rendering frame: %v", err)
				}
				time.Sleep(50 * time.Millisecond)
			}
			time.Sleep(500 * time.Millisecond)
			for i := 0; i < len(panel.Leds); i++ {
				panel.Leds[i] = RGBW{R: 0, G: 0, B: 0, W: 0}
			}
			ledData := make([]uint32, len(panel.Leds))
			if err := panel.Driver.Render(ledData); err != nil {
				log.Printf("Error rendering frame: %v", err)
			}
			time.Sleep(500 * time.Millisecond)
		},
		func(panel *HexagonPanel) {
			// Hexagon frame animation
			maxSize := float64(LUT_W) / 2 // Use half of the context width as max size
			for size := 0.0; size <= maxSize; size += maxSize / 10 {
				renderHexagonFrame(panel, size)
			}
			for size := maxSize; size >= 0.0; size -= maxSize / 10 {
				renderHexagonFrame(panel, size)
			}
			// Clear the panel after animation
			ledData := make([]uint32, len(panel.Leds))
			if err := panel.Driver.Render(ledData); err != nil {
				log.Printf("Error clearing panel: %v", err)
			}
		},
	}

	go func() {
		for {
			for _, animation := range animations {
				animation(panel)
			}
		}
	}()
}
