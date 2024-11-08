package comms

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

var debugSerial bool

func init() {
	debugSerial = os.Getenv("DEBUG_SERIAL") != ""
	fmt.Printf("DEBUG_SERIAL=%v\n", debugSerial)
}

func debugLog(format string, v ...interface{}) {
	if debugSerial {
		log.Printf(format, v...)
	}
}

type ScreenUpdate struct {
	DeviceID string
	Output   string
}

const (
	HANDSHAKE_INTERVAL = 3 * time.Second
)

type DeviceInfo struct {
	DeviceSerialNo string `json:"device_serial_no"`
	DeviceID       string `json:"device_id"`
	SerialPort     string `json:"serial_port"`
}

type SerialConnection struct {
	Port     serial.Port
	DeviceID string
	Output   string
	PortName string
}

func OpenSerialPort(port string, connections []*SerialConnection, connectionsMutex *sync.Mutex) (*SerialConnection, error) {
	var conn serial.Port
	var err error
	mode := &serial.Mode{
		BaudRate:          1000000,
		DataBits:          8,
		Parity:            serial.NoParity,
		StopBits:          serial.OneStopBit,
		InitialStatusBits: &serial.ModemOutputBits{},
	}

	for retries := 0; retries < 5; retries++ {
		debugLog("Attempting to open port %s (attempt %d)", port, retries+1)
		conn, err = serial.Open(port, mode)
		if err == nil {
			time.Sleep(time.Millisecond * 100)
			break
		}
		debugLog("Failed to open port %s: %v. Retrying...", port, err)
		time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open port %s after 5 attempts: %v", port, err)
	}

	serialConn := &SerialConnection{
		Port:     conn,
		DeviceID: "",
		Output:   "",
		PortName: port,
	}

	// Clear any existing data in the buffer
	conn.ResetInputBuffer()
	conn.ResetOutputBuffer()

	// Perform initial handshake
	// if err := PerformHandshake(serialConn); err != nil {
	// 	conn.Close()
	// 	return nil, fmt.Errorf("initial handshake failed for port %s: %v", port, err)
	// }

	// Start periodic handshake goroutine
	// go PeriodicHandshake(serialConn, connections, connectionsMutex)

	debugLog("Successfully opened and initialized port %s", port)
	return serialConn, nil
}

func PerformHandshake(conn *SerialConnection) error {
	for retries := 0; retries < 5; retries++ {
		debugLog("Sending handshake to %s (attempt %d)", conn.PortName, retries+1)
		_, err := conn.Port.Write([]byte("H\n"))
		if err != nil {
			debugLog("Failed to send handshake to %s: %v", conn.PortName, err)
			continue
		}

		response, err := waitForAnyResponse(conn, []string{"K", "HB"}, 10*time.Second)
		if err == nil {
			debugLog("Received valid handshake response %s from device on port %s", response, conn.PortName)
			return nil
		}
		debugLog("Handshake attempt %d failed for %s: %v", retries+1, conn.PortName, err)
		time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
	}
	return fmt.Errorf("handshake failed after 5 attempts for %s", conn.PortName)
}

func PeriodicHandshake(conn *SerialConnection, connections []*SerialConnection, connectionsMutex *sync.Mutex) {
	jitter := time.Duration(rand.Float64()*1000-500) * time.Millisecond
	ticker := time.NewTicker(HANDSHAKE_INTERVAL + jitter)
	defer ticker.Stop()

	for {
		<-ticker.C
		if err := PerformHandshake(conn); err != nil {
			debugLog("Handshake failed for device %s: %v", conn.DeviceID, err)
			go AttemptReconnection(conn, connections, connectionsMutex)
			return // Exit this goroutine, as reconnection will start a new one if successful
		}
	}
}

func waitForAnyResponse(conn *SerialConnection, expectedResponses []string, timeout time.Duration) (string, error) {
	startTime := time.Now()
	buffer := make([]byte, 128)
	debugLog("Starting to wait for response from %s (expecting: %v)", conn.PortName, expectedResponses)

	// Set read timeout for this operation
	conn.Port.SetReadTimeout(timeout)

	for time.Since(startTime) < timeout {
		n, err := conn.Port.Read(buffer)
		if err != nil {
			if err != io.EOF {
				debugLog("Error reading from %s: %v", conn.PortName, err)
			}
			return "", err
		}
		if n > 0 {
			// Add hex dump of received data
			debugLog("Raw data from %s: [%X] as string: [%s]",
				conn.PortName,
				buffer[:n],
				string(buffer[:n]))

			receivedData := string(buffer[:n])
			conn.Output += receivedData

			for _, expected := range expectedResponses {
				if strings.Contains(receivedData, expected) {
					return expected, nil
				}
			}
		}
	}
	return "", fmt.Errorf("timeout waiting for response from %s. Received data: %s", conn.PortName, conn.Output)
}

func ReadSerialOutput(conn *SerialConnection, deviceUpdateChan chan<- ScreenUpdate, connections []*SerialConnection, connectionsMutex *sync.Mutex) {
	reader := bufio.NewReader(conn.Port)
	buffer := make([]byte, 1024)

	for {
		n, err := reader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				debugLog("Error reading from %s: %v", conn.DeviceID, err)
			}
			go AttemptReconnection(conn, connections, connectionsMutex)
			return
		}

		if n > 0 {
			data := string(buffer[:n])
			debugLog("Received from device %s: %s", conn.DeviceID, data)

			lines := strings.Split(data, "\n")
			var filteredLines []string
			for _, line := range lines {
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine == "K" {
					deviceUpdateChan <- ScreenUpdate{DeviceID: conn.DeviceID, Output: "ACK\n"}
				} else if trimmedLine == "HB" {
					deviceUpdateChan <- ScreenUpdate{DeviceID: conn.DeviceID, Output: "HB\n"}
				} else if trimmedLine != "" {
					filteredLines = append(filteredLines, line)
				}
			}

			filteredData := strings.Join(filteredLines, "\n")
			if filteredData != "" {
				connectionsMutex.Lock()
				conn.Output += filteredData + "\n"
				connectionsMutex.Unlock()

				deviceUpdateChan <- ScreenUpdate{DeviceID: conn.DeviceID, Output: filteredData + "\n"}
			}
		}
	}
}

func AttemptReconnection(conn *SerialConnection, connections []*SerialConnection, connectionsMutex *sync.Mutex) {
	// Close the existing connection first
	if conn.Port != nil {
		conn.Port.Close()
	}

	for {
		debugLog("Attempting to reconnect to %s", conn.PortName)
		newConn, err := OpenSerialPort(conn.PortName, connections, connectionsMutex)
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
			debugLog("Successfully reconnected to %s", conn.PortName)
			*conn = *newConn // Update the original connection with the new one
			return
		}
		debugLog("Failed to reconnect to %s: %v", conn.PortName, err)
		time.Sleep(time.Second * 30)
	}
}
