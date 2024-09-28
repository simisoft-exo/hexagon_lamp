package comms

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tarm/serial"
)

var debugSerial bool

func init() {
	debugSerial = os.Getenv("DEBUG_SERIAL") == "true"
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
	HANDSHAKE_INTERVAL = 5 * time.Second
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

func OpenSerialPort(port string, connections []*SerialConnection, connectionsMutex *sync.Mutex) (*SerialConnection, error) {
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
		debugLog("Attempting to open port %s (attempt %d)", port, retries+1)
		conn, err = serial.OpenPort(config)
		if err == nil {
			break
		}
		debugLog("Failed to open port %s: %v. Retrying...", port, err)
		time.Sleep(time.Second * 2)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open port %s after 3 attempts: %v", port, err)
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
	if err := PerformHandshake(serialConn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("initial handshake failed for port %s: %v", port, err)
	}

	// Start periodic handshake goroutine
	go PeriodicHandshake(serialConn, connections, connectionsMutex)

	debugLog("Successfully opened and initialized port %s", port)
	return serialConn, nil
}

func PerformHandshake(conn *SerialConnection) error {
	for retries := 0; retries < 3; retries++ {
		_, err := conn.Port.Write([]byte("H\n"))
		if err != nil {
			debugLog("Failed to send handshake to %s: %v", conn.PortName, err)
			continue
		}

		response, err := waitForAnyResponse(conn, []string{"ACK", "HEARTBEAT"}, 30*time.Second)
		if err == nil {
			debugLog("Received %s from device on port %s", response, conn.PortName)
			return nil
		}
		debugLog("Handshake attempt %d failed for %s: %v", retries+1, conn.PortName, err)
		time.Sleep(time.Second * 2)
	}
	return fmt.Errorf("handshake failed after 3 attempts for %s", conn.PortName)
}

func PeriodicHandshake(conn *SerialConnection, connections []*SerialConnection, connectionsMutex *sync.Mutex) {
	ticker := time.NewTicker(HANDSHAKE_INTERVAL)
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
	for time.Since(startTime) < timeout {
		n, err := conn.Port.Read(buffer)
		if err != nil {
			if err != io.EOF {
				debugLog("Error reading from %s: %v", conn.DeviceID, err)
			}
			return "", err
		}
		if n > 0 {
			receivedData := string(buffer[:n])
			conn.Output += receivedData
			debugLog("Received data from %s: %s", conn.DeviceID, receivedData)
			for _, expected := range expectedResponses {
				if strings.Contains(receivedData, expected) {
					return expected, nil
				}
			}
		}
	}
	return "", fmt.Errorf("timeout waiting for response from %s. Received data: %s", conn.DeviceID, conn.Output)
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
				if trimmedLine == "ACK" {
					deviceUpdateChan <- ScreenUpdate{DeviceID: conn.DeviceID, Output: "ACK\n"}
				} else if trimmedLine == "HEARTBEAT" {
					deviceUpdateChan <- ScreenUpdate{DeviceID: conn.DeviceID, Output: "HEARTBEAT\n"}
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
