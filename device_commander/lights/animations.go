package lights

import (
	"log"
	"math"
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
	__               = -1
)

var LUT = [LUT_LEN]int{
	__, __, __, 0, __, 1, __, 2, __, __, __,
	__, __, 6, __, 5, __, 4, __, 3, __, __,
	__, __, 7, __, 8, __, 9, __, 10, __, __,
	__, 15, __, 14, __, 13, __, 12, __, 11, __,
	__, 16, __, 17, __, 18, __, 19, __, 20, __,
	26, __, 25, __, 24, __, 23, __, 22, __, 21,
	27, __, 28, __, 29, __, 30, __, 31, __, 32,
	__, 37, __, 36, __, 35, __, 34, __, 33, __,
	__, 38, __, 39, __, 40, __, 41, __, 42, __,
	__, __, 46, __, 45, __, 44, __, 43, __, __,
	__, __, 47, __, 48, __, 49, __, 50, __, __,
	__, __, __, 53, __, 52, __, 51, __, __, __,
}

func growingShrinkingHexagon(panel *HexagonPanel) {
	maxSize := float64(LUT_W)/2 + 1 // Maximum size of the hexagon

	log.Printf("Playing animation: growingShrinking hexagon: %f", maxSize)
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

	// Calculate color based on size (gradient from red to blue)
	r := uint8(255 * (size / (float64(LUT_W) / 2)))
	g := uint8(0)
	b := uint8(255 * (1 - (size / (float64(LUT_W) / 2))))
	DrawHexagon(dc, float64(LUT_W)/2, float64(LUT_H)/2, size, RGBW{R: r, G: g, B: b, W: 0})

	// Convert the drawing to LED data
	panel.DrawToPanel(dc)

	// Convert RGBW to uint32 for the LED driver
	ledData := make([]uint32, len(panel.Leds))
	for i, led := range panel.Leds {
		ledData[i] = uint32(led.R)<<24 | uint32(led.G)<<16 | uint32(led.B)<<8 | uint32(led.W)
	}

	// Render the frame
	if err := panel.Driver.Render(ledData); err != nil {
		log.Printf("Error rendering frame: %v", err)
	}

	// Add debug logging
	log.Printf("Rendered frame with size: %f, LED data: %v", size, ledData)

	time.Sleep(50 * time.Millisecond) // Adjust speed as needed
}

func (h *HexagonPanel) DrawToPanel(dc *gg.Context) {
	for y := 0; y < h.Height; y++ {
		for x := 0; x < h.Width; x++ {
			index := LUT[y*h.Width+x]
			if index != __ {
				r, g, b, a := dc.Image().At(x, y).RGBA()
				h.Leds[index] = RGBW{
					R: uint8(r >> 8),
					G: uint8(g >> 8),
					B: uint8(b >> 8),
					W: uint8(a >> 8),
				}
			}
		}
	}
}

func whiteLEDCascade(panel *HexagonPanel) {
	log.Println("Playing animation: White LED Cascade")
	// White LED Cascade animation
	for i := 0; i < len(panel.Leds); i++ {
		// Set each LED to full white (255 for all channels)
		panel.Leds[i] = RGBW{R: 120, G: 120, B: 120, W: 120}

		// Create a slice to hold LED data for rendering
		ledData := make([]uint32, len(panel.Leds))

		// Convert RGBW values to uint32 for each LED
		for j, led := range panel.Leds {
			ledData[j] = uint32(led.R)<<24 | uint32(led.G)<<16 | uint32(led.B)<<8 | uint32(led.W)
		}

		// Render the current frame
		if err := panel.Driver.Render(ledData); err != nil {
			log.Printf("Error rendering frame: %v", err)
		}

		// Pause briefly between each LED lighting up
		time.Sleep(200 * time.Millisecond)
	}

	// Pause after all LEDs are lit
	time.Sleep(2 * time.Second)
}

func offLEDCascade(panel *HexagonPanel) {
	log.Println("Playing animation: Turn Off LEDs One by One")

	// Create a slice to hold LED data for rendering
	ledData := make([]uint32, len(panel.Leds))

	// Turn off LEDs one by one
	for i := 0; i < len(ledData); i++ {
		// Turn off the current LED
		ledData[i] = 0 // All channels off

		// Render the current frame
		if err := panel.Driver.Render(ledData); err != nil {
			log.Printf("Error rendering frame: %v", err)
		}

		// Pause briefly between turning off each LED
		time.Sleep(200 * time.Millisecond)
	}

	// Pause after all LEDs are off
	time.Sleep(2 * time.Second)
}

func rainbowHueShift(panel *HexagonPanel) {
	log.Println("Playing animation: Rainbow Hue Shift")

	// Create a slice to hold LED data for rendering
	ledData := make([]uint32, len(panel.Leds))

	// Define the duration of one complete hue cycle
	cycleDuration := 180 * time.Second

	// Calculate the number of steps for a smooth transition
	steps := 500

	for step := 0; step < steps; step++ {
		// Calculate the current hue (0-360 degrees)
		hue := float64(step) / float64(steps) * 360.0

		// Convert HSV to RGB
		r, g, b := hsvToRgb(hue, 1.0, 1.0)

		// Set all LEDs to the current color
		for i := range ledData {
			ledData[i] = uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8 | uint32(0) // W channel is 0
		}

		// Render the current frame
		if err := panel.Driver.Render(ledData); err != nil {
			log.Printf("Error rendering frame: %v", err)
		}

		// Calculate sleep duration for smooth transition
		time.Sleep(cycleDuration / time.Duration(steps))
	}
}

// hsvToRgb converts HSV (Hue, Saturation, Value) to RGB
func hsvToRgb(h, s, v float64) (uint8, uint8, uint8) {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c

	var r, g, b float64

	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255)
}

func Run(panel *HexagonPanel) {
	// Define animation functions
	animations := []func(*HexagonPanel){
		// growingShrinkingHexagon,
		rainbowHueShift,
		offLEDCascade,
		whiteLEDCascade,
	}

	for {
		for _, animation := range animations {
			animation(panel)
		}
	}
}
