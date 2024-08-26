package main

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/tarm/serial"
)

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
	if err := performHandshake(serialConn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("initial handshake failed for port %s: %v", port, err)
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
	return fmt.Errorf("handshake failed after 3 attempts for %s", conn.PortName)
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
	return "", fmt.Errorf("timeout waiting for response from %s. Received data: %s", conn.DeviceID, conn.Output)
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
