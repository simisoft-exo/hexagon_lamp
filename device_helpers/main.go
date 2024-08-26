package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
)

type ScreenUpdate struct {
	DeviceID string
	Output   string
}

var (
	screenUpdateChan    = make(chan ScreenUpdate, 100) // Buffer for 100 updates
	screenRefreshTicker *time.Ticker
)

var (
	connections      []*SerialConnection
	connectionsMutex sync.Mutex
	currentPortIndex int
	screen           tcell.Screen
	inputBuffer      string
	logFile          *os.File
	sendToAllBuffer  string
)

func main() {
	// Set up logging
	var err error
	logFile, err = os.OpenFile("debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	// Device info for all 7 devices
	devicesJSON := `[
        {"device_serial_no": "0671FF383159503043112607", "device_id": "0", "serial_port": "/dev/ttyACM0"},
        {"device_serial_no": "066DFF515049657187212124", "device_id": "1", "serial_port": "/dev/ttyACM1"},
        {"device_serial_no": "066CFF383159503043112637", "device_id": "2", "serial_port": "/dev/ttyACM2"},
        {"device_serial_no": "066BFF515049657187203314", "device_id": "3", "serial_port": "/dev/ttyACM3"},
        {"device_serial_no": "066FFF383159503043114308", "device_id": "4", "serial_port": "/dev/ttyACM4"},
        {"device_serial_no": "066EFF383159503043112729", "device_id": "5", "serial_port": "/dev/ttyACM5"},
        {"device_serial_no": "066CFF383159503043112926", "device_id": "6", "serial_port": "/dev/ttyACM6"}
    ]`
	var devices []DeviceInfo
	err = json.Unmarshal([]byte(devicesJSON), &devices)
	if err != nil {
		log.Fatal(err)
	}

	connections = make([]*SerialConnection, 0, 7)

	var wg sync.WaitGroup
	connectionChan := make(chan *SerialConnection, len(devices))

	for _, device := range devices {
		wg.Add(1)
		go func(dev DeviceInfo) {
			defer wg.Done()
			conn, err := openSerialPort(dev.SerialPort)
			if err != nil {
				log.Printf("Failed to open %s: %v", dev.SerialPort, err)
				partialConn := &SerialConnection{
					DeviceID: dev.DeviceID,
					PortName: dev.SerialPort,
				}
				go attemptReconnection(partialConn)
				return
			}
			conn.DeviceID = dev.DeviceID
			conn.PortName = dev.SerialPort
			connectionChan <- conn
		}(device)
	}

	go func() {
		wg.Wait()
		close(connectionChan)
	}()

	for conn := range connectionChan {
		connections = append(connections, conn)
		go periodicHandshake(conn)
		go readSerialOutput(conn)
	}

	if len(connections) == 0 {
		log.Fatal("No serial connections were successfully opened")
	}

	screen, err = tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		log.Fatal(err)
	}
	defer screen.Fini()

	// Start the screen update goroutine
	go screenUpdateLoop()

	// Start the screen refresh ticker
	screenRefreshTicker = time.NewTicker(100 * time.Millisecond)
	defer screenRefreshTicker.Stop()

	drawScreen()
	for {
		select {
		case <-screenRefreshTicker.C:
			drawScreen()
		default:
			switch ev := screen.PollEvent().(type) {
			case *tcell.EventResize:
				screen.Sync()
				drawScreen()
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyEscape:
					return
				case tcell.KeyEnter:
					if currentPortIndex == len(connections) {
						sendCommandToAll(sendToAllBuffer, &connectionsMutex, connections)
						sendToAllBuffer = ""
					} else {
						sendCommand(inputBuffer, &connectionsMutex, connections, currentPortIndex)
						inputBuffer = ""
					}
				case tcell.KeyBackspace, tcell.KeyBackspace2:
					if currentPortIndex == len(connections) {
						if len(sendToAllBuffer) > 0 {
							sendToAllBuffer = sendToAllBuffer[:len(sendToAllBuffer)-1]
						}
					} else {
						if len(inputBuffer) > 0 {
							inputBuffer = inputBuffer[:len(inputBuffer)-1]
						}
					}
				case tcell.KeyTab:
					if ev.Modifiers() == tcell.ModShift {
						currentPortIndex = (currentPortIndex - 1 + len(connections) + 1) % (len(connections) + 1)
					} else {
						currentPortIndex = (currentPortIndex + 1) % (len(connections) + 1)
					}
				case tcell.KeyBacktab:
					currentPortIndex = (currentPortIndex - 1 + len(connections) + 1) % (len(connections) + 1)
				case tcell.KeyRune:
					if ev.Rune() == 'q' && ev.Modifiers() == tcell.ModAlt {
						return // Exit when Alt+Q is pressed
					}
					if currentPortIndex == len(connections) {
						sendToAllBuffer += string(ev.Rune())
					} else {
						inputBuffer += string(ev.Rune())
					}
				}
				drawScreen()
			}
		}
	}
}

func drawScreen() {
	if screen == nil {
		log.Println("Screen is nil in drawScreen()")
		return
	}

	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	screen.Clear()
	width, height := screen.Size()

	// Reserve 2 lines for input, 1 for "Send to All", and 1 for debug info
	availableHeight := height - 4

	// Sort connections by DeviceID
	sortedConnections := make([]*SerialConnection, len(connections))
	copy(sortedConnections, connections)
	sort.Slice(sortedConnections, func(i, j int) bool {
		return sortedConnections[i].DeviceID < sortedConnections[j].DeviceID
	})

	// Ensure at least 2 lines per device (1 for header, 1 for output)
	deviceHeight := max(2, availableHeight/len(sortedConnections))

	// Draw device info and outputs
	for i, conn := range sortedConnections {
		y := i * deviceHeight
		if y >= availableHeight {
			break // Stop if we've run out of space
		}

		// Draw device header
		headerText := fmt.Sprintf("Device ID: %s, Port: %s, Output Length: %d", conn.DeviceID, conn.PortName, len(conn.Output))
		if i == currentPortIndex {
			highlightText(0, y, width, headerText, tcell.ColorGreen, tcell.ColorBlack)
		} else {
			drawText(0, y, width, headerText)
		}

		// Draw device output
		lines := strings.Split(conn.Output, "\n")
		outputHeight := deviceHeight - 1 // Reserve one line for the header
		startLine := max(0, len(lines)-outputHeight)
		for j := 0; j < outputHeight && startLine+j < len(lines); j++ {
			if y+j+1 >= availableHeight {
				break
			}
			line := lines[startLine+j]
			drawText(0, y+j+1, width, line)
		}
	}

	// Draw input line
	inputLine := fmt.Sprintf("> %s", inputBuffer)
	drawText(0, height-3, width, inputLine)

	// Draw "Send to All" input line
	sendToAllLine := fmt.Sprintf("Send to All> %s", sendToAllBuffer)
	if currentPortIndex == len(sortedConnections) {
		highlightText(0, height-2, width, sendToAllLine, tcell.ColorGreen, tcell.ColorBlack)
	} else {
		drawText(0, height-2, width, sendToAllLine)
	}

	// Draw debug info
	debugInfo := fmt.Sprintf("Connected Devices: %d", len(sortedConnections))
	drawText(0, height-1, width, debugInfo)

	screen.Show()
}

func drawText(x, y, maxWidth int, text string) {
	for i, r := range []rune(text) {
		if i >= maxWidth {
			break
		}
		screen.SetContent(x+i, y, r, nil, tcell.StyleDefault)
	}
}

func highlightText(x, y, maxWidth int, text string, fg, bg tcell.Color) {
	style := tcell.StyleDefault.Foreground(fg).Background(bg)
	for i, r := range []rune(text) {
		if i >= maxWidth {
			break
		}
		screen.SetContent(x+i, y, r, nil, style)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const maxOutputLines = 100

func limitOutputBuffer(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > maxOutputLines {
		lines = lines[len(lines)-maxOutputLines:]
	}
	return strings.Join(lines, "\n")
}

func screenUpdateLoop() {
	for {
		select {
		case update := <-screenUpdateChan:
			// Update the connection's output
			connectionsMutex.Lock()
			for _, conn := range connections {
				if conn.DeviceID == update.DeviceID {
					conn.Output = limitOutputBuffer(conn.Output + update.Output)
					break
				}
			}
			connectionsMutex.Unlock()
		case <-screenRefreshTicker.C:
			drawScreen()
		}
	}
}
