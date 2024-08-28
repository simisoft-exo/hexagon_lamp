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
	"log"
	"time"

	ws2811 "github.com/rpi-ws281x/rpi-ws281x-go"
)

const (
	brightness = 255
	ledCounts  = 54
	sleepTime  = 200
)

type wsEngine interface {
	Init() error
	Render() error
	Wait() error
	Fini()
	Leds(channel int) []uint32
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

type colorWipe struct {
	ws wsEngine
}

func (cw *colorWipe) setup() error {
	return cw.ws.Init()
}

func (cw *colorWipe) display(color uint32) error {
	for i := 0; i < len(cw.ws.Leds(0)); i++ {
		cw.ws.Leds(0)[i] = color
		if err := cw.ws.Render(); err != nil {
			return err
		}
		time.Sleep(sleepTime * time.Millisecond)
	}
	return nil
}

func Run() {
	opt := ws2811.DefaultOptions
	opt.Channels[0].Brightness = brightness
	opt.Channels[0].LedCount = ledCounts
	opt.Channels[0].StripeType = ws2811.SK6812StripGRBW
	opt.Channels[0].GpioPin = 12 // Set GPIO pin to 12

	dev, err := ws2811.MakeWS2811(&opt)
	if err != nil {
		log.Fatalf("leds: Error creating WS2811: %v", err)
	}

	cw := &colorWipe{
		ws: dev,
	}

	if err := cw.setup(); err != nil {
		log.Fatalf("leds: Error setting up colorWipe: %v", err)
	}
	defer dev.Fini()
	// // Display blue
	// cw.display(uint32(0x0000ff))
	// // Display green
	// cw.display(uint32(0x00ff00))
	// // Display red
	// cw.display(uint32(0xff0000))
	// // Display off (black)
	// cw.display(uint32(0x000000))
	// // Display warm white (RGBW format)
	// cw.display(uint32(0x000000FF))

	// Display all white
	cw.display(uint32(0xFFFFFFFF))
}
