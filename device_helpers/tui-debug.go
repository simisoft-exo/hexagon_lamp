package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
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
				// Create a partial connection object for reconnection attempts
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

	// Start a goroutine for screen updates
	updateChan := make(chan struct{})
	go func() {
		for range updateChan {
			drawScreen()
		}
	}()

	drawScreen()
	for {
		switch ev := screen.PollEvent().(type) {
		case *tcell.EventResize:
			screen.Sync()
			updateChan <- struct{}{}
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
			updateChan <- struct{}{}
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
		_, err := conn.Port.Write([]byte("HANDSHAKE\n"))
		if err != nil {
			log.Printf("Failed to send handshake to %s: %v", conn.PortName, err)
			continue
		}

		response, err := waitForAnyResponse(conn, []string{"ACK_HANDSHAKE", "HEARTBEAT"}, 30*time.Second)
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

			conn.Output += received
			if screen != nil {
				screen.PostEvent(tcell.NewEventInterrupt(nil))
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

func sendInitialCommands(conn *SerialConnection) {
	commands := []string{"init"}
	for _, cmd := range commands {
		sendCommandWithRetry(conn, cmd, 3) // Try each command up to 3 times

		// Wait for acknowledgements
		initializingAck := "ACK_INIT"
		waitingForCommandAck := "ACK_RUNNING"

		if !waitForResponse(conn, initializingAck) {
			log.Printf("Did not receive INITIALIZING acknowledgement from device %s", conn.DeviceID)
			continue // Try the next command instead of returning
		}
		if !waitForResponse(conn, waitingForCommandAck) {
			log.Printf("Did not receive RUNNING acknowledgement from device %s", conn.DeviceID)
			continue // Try the next command instead of returning
		}
	}
}

func sendCommandWithRetry(conn *SerialConnection, command string, retries int) {
	for i := 0; i < retries; i++ {
		log.Printf("Sending command to %s (attempt %d): %s", conn.DeviceID, i+1, command)
		_, err := conn.Port.Write([]byte(command + "\n"))
		if err != nil {
			log.Printf("Error sending command to %s: %v", conn.DeviceID, err)
			time.Sleep(time.Second) // Wait before retry
			continue
		}
		return // Command sent successfully
	}
	log.Printf("Command %s failed after %d attempts for device %s", command, retries, conn.DeviceID)
}

func waitForResponse(conn *SerialConnection, expectedAck string) bool {
	startTime := time.Now()
	buffer := make([]byte, 128)
	for time.Since(startTime) < 10*time.Second {
		n, err := conn.Port.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from %s: %v", conn.DeviceID, err)
			}
			return false
		}
		if n > 0 {
			receivedData := string(buffer[:n])
			conn.Output += receivedData
			if strings.Contains(conn.Output, expectedAck) {
				return true
			} else if strings.Contains(conn.Output, "FOC_STATUS:") {
				status := strings.TrimSpace(strings.Split(conn.Output, "FOC_STATUS:")[1])
				log.Printf("FOC Status for device %s: %s", conn.DeviceID, status)
				if status != "MOT_INIT" {
					log.Printf("FOC initialization did not result in MOTOR_READY status for device %s", conn.DeviceID)
					return false
				}
			} else if strings.Contains(conn.Output, "INIT_FAILED") {
				log.Printf("Initialization failed for device %s", conn.DeviceID)
				return false
			} else {
				log.Printf("Received unexpected data from %s: %s", conn.DeviceID, receivedData)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Printf("Timeout waiting for acknowledgment from %s. Received data: %s", conn.DeviceID, conn.Output)
	return false
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

	// Ensure at least 2 lines per device (1 for header, 1 for output)
	deviceHeight := max(2, availableHeight/len(connections))

	// Draw device info and outputs
	for i, conn := range connections {
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
	if currentPortIndex == len(connections) {
		highlightText(0, height-2, width, sendToAllLine, tcell.ColorGreen, tcell.ColorBlack)
	} else {
		drawText(0, height-2, width, sendToAllLine)
	}

	// Draw debug info
	debugInfo := fmt.Sprintf("Connected Devices: %d", len(connections))
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
