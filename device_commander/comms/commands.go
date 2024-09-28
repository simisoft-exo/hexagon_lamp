package comms

import (
	"io"
	"log"
	"sync"
	"time"
)

func SendCommandToAll(command string, connectionsMutex *sync.Mutex, connections []*SerialConnection) {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	log.Printf("Sending command to all devices: %s", command)
	for _, conn := range connections {
		sendWithRetry(conn, command)
	}
}

func sendWithRetry(conn *SerialConnection, command string) {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		_, err := conn.Port.Write([]byte(command + "\n"))
		if err == nil {
			log.Printf("Command sent successfully to %s", conn.DeviceID)
			return
		}
		log.Printf("Error sending command to %s (attempt %d): %v", conn.DeviceID, i+1, err)
		time.Sleep(time.Millisecond * 100)
	}
	log.Printf("Failed to send command to %s after %d attempts", conn.DeviceID, maxRetries)
}

func SendCommand(command string, connectionsMutex *sync.Mutex, connections []*SerialConnection, currentPortIndex int) {
	connectionsMutex.Lock()
	conn := connections[currentPortIndex]
	connectionsMutex.Unlock()

	log.Printf("Sending command to %s: %s", conn.DeviceID, command)

	// Clear existing output
	connectionsMutex.Lock()
	conn.Output = ""
	connectionsMutex.Unlock()

	// Send command with retry
	sendWithRetry(conn, command)

	// Wait for a short time to ensure the command is processed
	time.Sleep(100 * time.Millisecond)

	// Read any immediate response with a timeout
	responseChan := make(chan string)
	errorChan := make(chan error)

	go func() {
		buffer := make([]byte, 1024)
		n, err := conn.Port.Read(buffer)
		if err != nil {
			errorChan <- err
			return
		}
		responseChan <- string(buffer[:n])
	}()

	select {
	case response := <-responseChan:
		log.Printf("Immediate response from %s: %s", conn.DeviceID, response)
		connectionsMutex.Lock()
		conn.Output += response
		connectionsMutex.Unlock()
	case err := <-errorChan:
		if err != io.EOF {
			log.Printf("Error reading response from %s: %v", conn.DeviceID, err)
		}
	case <-time.After(200 * time.Millisecond):
		log.Printf("Timeout waiting for immediate response from %s", conn.DeviceID)
	}
}
