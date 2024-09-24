// Copyright 2018 Jacques Supcik / HEIA-FR
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lights

import (
	"fmt"
	"log"
	"time"

	ws2811 "github.com/rpi-ws281x/rpi-ws281x-go"
)

const (
	brightness = 255
	ledCounts  = 54
	sleepTime  = 200
	gpioPin    = 12
	stripType  = ws2811.SK6812StripGRBW
)

type LEDDriver struct {
	ws *ws2811.WS2811
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

func makeDriver() (*LEDDriver, error) {
	opt := ws2811.DefaultOptions
	opt.Channels[0].Brightness = brightness
	opt.Channels[0].LedCount = ledCounts
	opt.Channels[0].GpioPin = gpioPin
	opt.Channels[0].StripeType = stripType

	dev, err := ws2811.MakeWS2811(&opt)
	if err != nil {
		return nil, err
	}

	driver := &LEDDriver{
		ws: dev,
	}

	if err := driver.ws.Init(); err != nil {
		return nil, err
	}

	return driver, nil
}

func (d *LEDDriver) Close() {
	d.ws.Fini()
}

func (d *LEDDriver) Render(data []uint32) error {
	if len(data) != ledCounts {
		return fmt.Errorf("invalid data length: expected %d, got %d", ledCounts, len(data))
	}

	copy(d.ws.Leds(0), data)
	return d.ws.Render()
}

// Initialize the LED panel
func InitializeLEDs(panel *HexagonPanel) {
	// Create a slice to hold LED data
	ledData := make([]uint32, 54)

	// Set all LEDs to off (black)
	for i := range ledData {
		ledData[i] = 0 // RGBW: 0x00000000 (all channels off)
	}

	// Render the initial state
	if err := panel.Driver.Render(ledData); err != nil {
		log.Printf("Error rendering initial LED state: %v", err)
	}

	// Short delay to ensure LEDs are initialized
	time.Sleep(100 * time.Millisecond)
}
