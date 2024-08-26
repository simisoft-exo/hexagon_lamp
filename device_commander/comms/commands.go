package comms

import (
	"log"
	"sync"
)

func SendCommandToAll(command string, connectionsMutex *sync.Mutex, connections []*SerialConnection) {
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

func SendCommand(command string, connectionsMutex *sync.Mutex, connections []*SerialConnection, currentPortIndex int) {
	connectionsMutex.Lock()
	conn := connections[currentPortIndex]
	connectionsMutex.Unlock()

	log.Printf("Sending command to %s: %s", conn.DeviceID, command)
	_, err := conn.Port.Write([]byte(command + "\n"))
	if err != nil {
		log.Printf("Error sending command to %s: %v", conn.DeviceID, err)
	}
}
