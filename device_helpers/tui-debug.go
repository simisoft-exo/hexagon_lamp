package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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
	KEEPALIVE_INTERVAL = 3 * time.Second
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

	for i, device := range devices {
		log.Printf("Attempting to open device %d: %s", i, device.SerialPort)
		conn, err := openSerialPort(device.SerialPort)
		if err != nil {
			log.Printf("Failed to open %s: %v", device.SerialPort, err)

			// List the process using the serial port
			out, err := exec.Command("sudo", "lsof", device.SerialPort).Output()
			if err != nil {
				log.Printf("Failed to list process using %s: %v", device.SerialPort, err)
				continue
			}
			output := string(out)
			lines := strings.Split(output, "\n")
			if len(lines) > 1 {
				fields := strings.Fields(lines[1])
				if len(fields) > 1 {
					pid := fields[1]
					log.Printf("Process using %s: PID %s", device.SerialPort, pid)

					// Kill the process
					cmd := exec.Command("sudo", "kill", "-9", pid)
					if err := cmd.Run(); err != nil {
						log.Printf("Failed to kill process with PID %s: %v", pid, err)
						continue
					}
					log.Printf("Killed process with PID %s", pid)
				}
			}

			// Release the serial port
			cmd := exec.Command("sudo", "fuser", "-k", device.SerialPort)
			if err := cmd.Run(); err != nil {
				log.Printf("Failed to release %s: %v", device.SerialPort, err)
				continue
			}
			log.Printf("Released %s", device.SerialPort)

			// Retry opening the serial port
			conn, err = openSerialPort(device.SerialPort)
			if err != nil {
				log.Printf("Failed to open %s after releasing: %v", device.SerialPort, err)
				continue
			}
		}
		log.Printf("Successfully opened device %d: %s", i, device.SerialPort)
		conn.DeviceID = device.DeviceID
		connectionsMutex.Lock()
		connections = append(connections, conn)
		connectionsMutex.Unlock()
		go readSerialOutput(len(connections) - 1)
		// log.Printf("Sending initial commands to device %d", i)
		// sendInitialCommands(connection)
		// time.Sleep(time.Second * 3)
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

	drawScreen()
	for {
		switch ev := screen.PollEvent().(type) {
		case *tcell.EventResize:
			screen.Sync()
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
		}
		drawScreen()
	}
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

func openSerialPort(port string) (*SerialConnection, error) {
	config := &serial.Config{
		Name:        port,
		Baud:        1000000,
		ReadTimeout: time.Second * 10,
		Size:        8,
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
	}
	log.Printf("Opening port %s with config: %+v", port, config)
	conn, err := serial.OpenPort(config)
	if err != nil {
		return nil, err
	}

	// Create a SerialConnection
	serialConn := &SerialConnection{
		Port:     conn,
		DeviceID: "", // Set the appropriate device ID
		Output:   "",
		PortName: port,
	}

	// Perform handshake
	_, err = serialConn.Port.Write([]byte("H"))
	if err != nil {
		serialConn.Port.Close()
		return nil, err
	}

	// Wait for handshake acknowledgment
	if waitForResponse(serialConn, "ACK_HANDSHAKE") {
		log.Printf("Handshake successful with Arduino on port %s", port)
	} else {
		serialConn.Port.Close()
		return nil, fmt.Errorf("Failed to receive handshake acknowledgment from Arduino")
	}

	// Start keepalive goroutine
	go keepAlive(serialConn)

	return serialConn, nil
}

// Add this new function for keepalive
func keepAlive(conn *SerialConnection) {
	ticker := time.NewTicker(KEEPALIVE_INTERVAL)
	defer ticker.Stop()

	for {
		<-ticker.C
		_, err := conn.Port.Write([]byte("KEEPALIVE\n"))
		if err != nil {
			log.Printf("Error sending keepalive to %s: %v", conn.DeviceID, err)
			continue
		}

		if !waitForResponse(conn, "ACK_KEEPALIVE") {
			log.Printf("Did not receive keepalive acknowledgment from device %s", conn.DeviceID)
			// Attempt to reconnect or handle disconnection
			// This could involve closing the current connection and reopening it
			// You might want to implement a reconnection mechanism here
		}
	}
}

func readSerialOutput(index int) {
	connectionsMutex.Lock()
	conn := connections[index]
	connectionsMutex.Unlock()

	buffer := make([]byte, 128)
	for {
		n, err := conn.Port.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from %s: %v", conn.DeviceID, err)
			}
			// Handle disconnection
			log.Printf("Device %s disconnected, attempting to reconnect...", conn.DeviceID)
			// Implement reconnection logic here
			// This could involve closing the current connection and reopening it
			return
		}
		if n > 0 {
			received := string(buffer[:n])
			log.Printf("Received %d bytes from device %s: %s", n, conn.DeviceID, received)
			conn.Output += received
			drawScreen()
		}
	}
}

func sendInitialCommands(conn *SerialConnection) {
	commands := []string{"init"}
	for _, cmd := range commands {
		sendCommandWithRetry(conn, cmd, 3) // Try each command up to 3 times
		// time.Sleep(time.Second * 1)        // Wait between commands

		// Wait for acknowledgements
		initializingAck := fmt.Sprintf("ACK_INIT: INITIALIZING")
		waitingForCommandAck := fmt.Sprintf("ACK_INIT: WAITING_FOR_COMMAND")

		if !waitForResponse(conn, initializingAck) {
			log.Printf("Did not receive INITIALIZING acknowledgement from device %s", conn.DeviceID)
			return
		}
		if !waitForResponse(conn, waitingForCommandAck) {
			log.Printf("Did not receive WAITING_FOR_COMMAND acknowledgement from device %s", conn.DeviceID)
			return
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

		time.Sleep(time.Second) // Wait before retry
	}
	log.Printf("Command %s failed after %d attempts for device %s", command, retries, conn.DeviceID)
}

func waitForResponse(conn *SerialConnection, expectedAck string) bool {
	startTime := time.Now()
	buffer := make([]byte, 128)
	for time.Since(startTime) < 5*time.Second {
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
			} else {
				log.Printf("Received unexpected data from %s: %s", conn.DeviceID, receivedData)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Printf("Timeout waiting for acknowledgment from %s. Received data: %s", conn.DeviceID, conn.Output)
	return false
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
