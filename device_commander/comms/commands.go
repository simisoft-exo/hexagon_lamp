package comms

import (
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
	sendWithRetry(conn, command)
}
