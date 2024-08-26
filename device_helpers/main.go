package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/tarm/serial"
)

type DeviceInfo struct {
	DeviceSerialNo string `json:"device_serial_no"`
	DeviceID       string `json:"device_id"`
	SerialPort     string `json:"serial_port"`
}

type SerialConnection struct {
	Port     *serial.Port
	DeviceID string
	Output   string
	PortName string
}

const (
	HANDSHAKE_INTERVAL = 5 * time.Second
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
						sendCommandToAll(sendToAllBuffer)
						sendToAllBuffer = ""
					} else {
						sendCommand(inputBuffer)
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
func openSerialPort(port string) (*SerialConnection, error) {
	var conn *serial.Port
	var err error
	for retries := 0; retries < 3; retries++ {
		config := &serial.Config{
			Name:        port,
			Baud:        1000000,
			ReadTimeout: time.Second * 30, // Increase timeout
			Size:        8,
			Parity:      serial.ParityNone,
			StopBits:    serial.Stop1,
		}
		log.Printf("Attempting to open port %s (attempt %d)", port, retries+1)
		conn, err = serial.OpenPort(config)
		if err == nil {
			break
		}
		log.Printf("Failed to open port %s: %v. Retrying...", port, err)
		time.Sleep(time.Second * 2)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to open port %s after 3 attempts: %v", port, err)
	}

	serialConn := &SerialConnection{
		Port:     conn,
		DeviceID: "", // This will be set later
		Output:   "",
		PortName: port,
	}

	// Clear any existing data in the buffer
	conn.Flush()

	// Perform initial handshake
	if err := performHandshake(serialConn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Initial handshake failed for port %s: %v", port, err)
	}

	// Start periodic handshake goroutine
	go periodicHandshake(serialConn)

	// Start reading output
	go readSerialOutput(serialConn)

	log.Printf("Successfully opened and initialized port %s", port)
	return serialConn, nil
}

func performHandshake(conn *SerialConnection) error {
	for retries := 0; retries < 3; retries++ {
		_, err := conn.Port.Write([]byte("H\n"))
		if err != nil {
			log.Printf("Failed to send handshake to %s: %v", conn.PortName, err)
			continue
		}

		response, err := waitForAnyResponse(conn, []string{"ACK", "HEARTBEAT"}, 30*time.Second)
		if err == nil {
			log.Printf("Received %s from device on port %s", response, conn.PortName)
			return nil
		}
		log.Printf("Handshake attempt %d failed for %s: %v", retries+1, conn.PortName, err)
		time.Sleep(time.Second * 2)
	}
	return fmt.Errorf("Handshake failed after 3 attempts for %s", conn.PortName)
}

func periodicHandshake(conn *SerialConnection) {
	ticker := time.NewTicker(HANDSHAKE_INTERVAL)
	defer ticker.Stop()

	for {
		<-ticker.C
		if err := performHandshake(conn); err != nil {
			log.Printf("Handshake failed for device %s: %v", conn.DeviceID, err)
			go attemptReconnection(conn)
			return // Exit this goroutine, as reconnection will start a new one if successful
		}
	}
}

func waitForAnyResponse(conn *SerialConnection, expectedResponses []string, timeout time.Duration) (string, error) {
	startTime := time.Now()
	buffer := make([]byte, 128)
	for time.Since(startTime) < timeout {
		n, err := conn.Port.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from %s: %v", conn.DeviceID, err)
			}
			return "", err
		}
		if n > 0 {
			receivedData := string(buffer[:n])
			conn.Output += receivedData
			log.Printf("Received data from %s: %s", conn.DeviceID, receivedData)
			for _, expected := range expectedResponses {
				if strings.Contains(receivedData, expected) {
					return expected, nil
				}
			}
		}
	}
	return "", fmt.Errorf("Timeout waiting for response from %s. Received data: %s", conn.DeviceID, conn.Output)
}

func readSerialOutput(conn *SerialConnection) {
	buffer := make([]byte, 128)
	for {
		if conn == nil || conn.Port == nil {
			log.Printf("Connection or port is nil for device %s", conn.DeviceID)
			go attemptReconnection(conn)
			return // Exit this goroutine, as reconnection will start a new one if successful
		}

		n, err := conn.Port.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from %s: %v", conn.DeviceID, err)
			}
			go attemptReconnection(conn)
			return // Exit this goroutine, as reconnection will start a new one if successful
		}
		if n > 0 {
			received := string(buffer[:n])
			log.Printf("Received %d bytes from device %s: %s", n, conn.DeviceID, received)

			if strings.Contains(received, "HEARTBEAT") {
				log.Printf("Heartbeat received from device %s", conn.DeviceID)
			}

			// Send update to channel
			select {
			case screenUpdateChan <- ScreenUpdate{DeviceID: conn.DeviceID, Output: received}:
			default:
				// Channel is full, log a warning
				log.Printf("Warning: Screen update channel is full. Update for device %s dropped.", conn.DeviceID)
			}
		}
	}
}

func attemptReconnection(conn *SerialConnection) {
	for {
		log.Printf("Attempting to reconnect to %s", conn.PortName)
		newConn, err := openSerialPort(conn.PortName)
		if err == nil {
			newConn.DeviceID = conn.DeviceID
			connectionsMutex.Lock()
			for i, c := range connections {
				if c.DeviceID == conn.DeviceID {
					connections[i] = newConn
					break
				}
			}
			connectionsMutex.Unlock()
			log.Printf("Successfully reconnected to %s", conn.PortName)
			*conn = *newConn // Update the original connection with the new one
			return
		}
		log.Printf("Failed to reconnect to %s: %v", conn.PortName, err)
		time.Sleep(time.Second * 30)
	}
}

func reconnect(conn *SerialConnection) error {
	log.Printf("Attempting to reconnect to device %s on port %s", conn.DeviceID, conn.PortName)

	// Close the existing connection if it's still open
	if conn.Port != nil {
		conn.Port.Close()
	}

	// Attempt to reopen the port
	config := &serial.Config{
		Name:        conn.PortName,
		Baud:        1000000,
		ReadTimeout: time.Second * 10,
		Size:        8,
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
	}

	port, err := serial.OpenPort(config)
	if err != nil {
		return fmt.Errorf("Failed to reopen port %s: %v", conn.PortName, err)
	}

	conn.Port = port

	return performHandshake(conn)
}

func sendCommandToAll(command string) {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	log.Printf("Sending command to all devices: %s", command)
	for _, conn := range connections {
		_, err := conn.Port.Write([]byte(command + "\n"))
		if err != nil {
			log.Printf("Error sending command to %s: %v", conn.DeviceID, err)
		}
	}
}

func sendCommand(command string) {
	connectionsMutex.Lock()
	conn := connections[currentPortIndex]
	connectionsMutex.Unlock()

	log.Printf("Sending command to %s: %s", conn.DeviceID, command)
	_, err := conn.Port.Write([]byte(command + "\n"))
	if err != nil {
		log.Printf("Error sending command to %s: %v", conn.DeviceID, err)
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
